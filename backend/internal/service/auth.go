// Package service holds business logic. It owns transactions, returns domain
// errors, and knows nothing about HTTP.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/model"
)

// userFinder is the slice of the user repository this service actually uses.
//
// The interface is declared HERE, by the consumer, not by the repository that
// implements it. That is inverted from Spring, where the repository interface
// is the artifact and the service depends on it. Two things fall out of it:
// the service depends on one method rather than a whole repository type, and a
// test can satisfy it with a five-line struct — no mocking framework, no
// generated doubles.
type userFinder interface {
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

// tokenIssuer is the same idea for JWT minting: the service says what it needs,
// and *util.JWT happens to satisfy it.
type tokenIssuer interface {
	GenerateToken(userID uuid.UUID, role model.Role) (string, error)
}

type AuthService struct {
	users  userFinder
	tokens tokenIssuer
}

func NewAuthService(users userFinder, tokens tokenIssuer) *AuthService {
	return &AuthService{users: users, tokens: tokens}
}

// ErrInvalidCredentials is the ONLY error a failed login returns, whether the
// email is unknown or the password is wrong.
//
// This is the requirement that the two cases be indistinguishable. Returning
// "no such user" for one and "wrong password" for the other turns the login
// endpoint into an account-enumeration oracle: an attacker feeds it an address
// list and learns exactly who has an account here, which is worth money on its
// own and narrows every later attack.
//
// Because handler.HandleError derives the response from the error, one error
// value means one byte-identical 401 for both paths — the status, the code and
// the message all come from here.
var ErrInvalidCredentials = domain.Unauthorizedf("invalid email or password")

// dummyHash is a real bcrypt hash of a throwaway password, compared against when
// no user matches the email.
//
// Without it the two failure paths take visibly different amounts of time: an
// unknown email returns as soon as the query misses (~1ms), while a wrong
// password pays for a full bcrypt comparison (~60ms). That gap is measurable
// over the network and leaks exactly what the identical response was meant to
// hide. Doing the same work in both branches closes it.
//
// Generated at startup rather than hardcoded so it always uses the same cost
// parameter as the real hashes it is standing in for; a hardcoded hash would
// silently stop matching if the cost were ever raised.
var dummyHash = mustGenerateDummyHash()

func mustGenerateDummyHash() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)
	if err != nil {
		// Only fails on an out-of-range cost, which is a compile-time constant
		// here. If it ever does fail, the process must not start with the
		// timing defence silently disabled.
		panic(fmt.Sprintf("generating dummy bcrypt hash: %v", err))
	}
	return h
}

// Login verifies an email and password and returns a signed token.
//
// Every failure returns ErrInvalidCredentials. Resist the urge to be helpful
// here: "that account is locked", "unknown email", and "wrong password" are all
// information an attacker wants and a legitimate user does not need.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Burn the same time a real comparison would, then fail identically.
			// The result is deliberately discarded — it can never match.
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return "", ErrInvalidCredentials
		}
		// A genuine database failure is NOT a credential problem. Letting it
		// through as a 401 would tell the user their password is wrong when the
		// database is simply down, and would hide the outage from the logs.
		return "", fmt.Errorf("looking up user for login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	token, err := s.tokens.GenerateToken(user.ID, user.Role)
	if err != nil {
		return "", fmt.Errorf("issuing token for user %s: %w", user.ID, err)
	}

	return token, nil
}

// ErrStaleSession is returned when a token is validly signed but names a user
// that no longer exists.
//
// 401 rather than the 404 that domain.ErrNotFound would produce. The row being
// gone is not a missing page the caller asked for — it means the credential
// they presented no longer identifies anyone, and the honest instruction is to
// log in again.
var ErrStaleSession = domain.Unauthorizedf("session is no longer valid")

// CurrentUser loads the user a validated token names.
//
// It reads the row instead of trusting the token's claims. A token is a
// snapshot taken at login and valid for a day: by now the account may have been
// deleted, renamed, or demoted from admin to customer. Anything derived from
// the claims alone would keep reporting what was true at login.
func (s *AuthService) CurrentUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrStaleSession
		}
		return nil, fmt.Errorf("loading current user %s: %w", id, err)
	}
	return user, nil
}
