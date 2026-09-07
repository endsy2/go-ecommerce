// Package util holds small, dependency-light helpers that do not belong to a
// layer. It is a leaf: nothing in here may import handler, service or
// repository, or the layering in backend/CLAUDE.md stops meaning anything.
package util

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"ecommerce/backend/internal/model"
)

// Claims is the payload carried by an issued token.
//
// RegisteredClaims is embedded, which supplies the standard fields the task
// asked for: sub (Subject), exp (ExpiresAt) and iat (IssuedAt). Role is the one
// custom claim, so it is the only field declared here.
//
// Everything in a JWT is signed but NOT encrypted — it is base64, not
// ciphertext, and any client can read it. Never put anything secret in here.
type Claims struct {
	Role model.Role `json:"role"`

	jwt.RegisteredClaims
}

// JWT issues and validates tokens.
//
// It is a struct rather than a set of package-level functions because the
// signing secret has to live somewhere, and a package-level variable would be
// global mutable state that tests cannot vary independently. This keeps the
// project's manual-wiring style: main.go constructs it, and whoever needs it
// receives it.
type JWT struct {
	secret []byte
	expiry time.Duration
}

// NewJWT builds an issuer. Callers pass cfg.JWT.Secret and cfg.JWT.Expiry.
func NewJWT(secret []byte, expiry time.Duration) *JWT {
	return &JWT{secret: secret, expiry: expiry}
}

// GenerateToken signs a token for the given user, valid for the configured
// expiry window.
func (j *JWT) GenerateToken(userID uuid.UUID, role model.Role) (string, error) {
	now := time.Now()

	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return signed, nil
}

// ErrInvalidToken is returned for every rejection reason: bad signature, wrong
// algorithm, expired, malformed, or a subject that is not a uuid.
//
// One error for all of them on purpose. A caller that could tell "expired" from
// "forged" would be tempted to report the difference, and telling an attacker
// which part of their forgery failed is free information.
var ErrInvalidToken = errors.New("invalid token")

// ValidateToken verifies the signature and the time claims, returning the
// payload only if everything checks out.
func (j *JWT) ValidateToken(raw string) (*Claims, error) {
	claims := &Claims{}

	// WithValidMethods is the important argument, not a nicety. Without it the
	// library would accept whatever "alg" the token itself declares — including
	// "none", which means an attacker writes their own claims, sends no
	// signature, and is admitted. Pinning the algorithm to the one we sign with
	// is what closes that whole family of attacks.
	_, err := jwt.ParseWithClaims(raw, claims,
		func(t *jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		// Wrapped, so a caller can still errors.Is(err, ErrInvalidToken) while
		// the underlying reason stays available for a debug log.
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// The signature only proves we minted the token, not that its contents are
	// still meaningful. A subject that will not parse is a token we should never
	// have issued, so treat it as invalid rather than handing a zero uuid on.
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, fmt.Errorf("%w: subject is not a uuid", ErrInvalidToken)
	}

	return claims, nil
}

// UserID returns the subject parsed as a uuid. Safe to call on claims that came
// back from ValidateToken, which has already checked it parses.
func (c *Claims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}
