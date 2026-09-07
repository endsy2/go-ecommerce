package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/model"
)

// The fakes below are the whole reason the service declares its own interfaces.
// Each is a few lines of ordinary Go: no mocking framework, no code generation,
// no expectation-setting DSL.

type fakeUserFinder struct {
	user *model.User
	err  error
}

func (f fakeUserFinder) FindByEmail(context.Context, string) (*model.User, error) {
	return f.user, f.err
}

// FindByID resolves to the same fixture as FindByEmail. One pair of fields
// backs both lookups because no test here needs them to disagree, and a second
// pair would only be state to keep in sync.
func (f fakeUserFinder) FindByID(context.Context, uuid.UUID) (*model.User, error) {
	return f.user, f.err
}

type fakeTokenIssuer struct {
	token string
	err   error
}

func (f fakeTokenIssuer) GenerateToken(uuid.UUID, model.Role) (string, error) {
	return f.token, f.err
}

// testUser builds a user whose stored hash really is bcrypt of password, so the
// comparison under test is the real one rather than a stub.
func testUser(t *testing.T, password string) *model.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing test password: %v", err)
	}

	u := &model.User{Email: "bob@example.com", PasswordHash: string(hash), Role: model.RoleCustomer}
	u.ID = uuid.New()
	return u
}

func TestAuthServiceLogin(t *testing.T) {
	user := testUser(t, "correct-horse")

	tests := []struct {
		name      string
		finder    fakeUserFinder
		issuer    fakeTokenIssuer
		password  string
		wantToken string
		wantErr   error
	}{
		{
			name:      "valid credentials return a token",
			finder:    fakeUserFinder{user: user},
			issuer:    fakeTokenIssuer{token: "signed.jwt.value"},
			password:  "correct-horse",
			wantToken: "signed.jwt.value",
		},
		{
			name:     "unknown email is rejected as invalid credentials",
			finder:   fakeUserFinder{err: domain.NotFoundf("user not found")},
			password: "correct-horse",
			wantErr:  ErrInvalidCredentials,
		},
		{
			name:     "wrong password is rejected as invalid credentials",
			finder:   fakeUserFinder{user: user},
			password: "not-the-password",
			wantErr:  ErrInvalidCredentials,
		},
		{
			name:     "empty password is rejected as invalid credentials",
			finder:   fakeUserFinder{user: user},
			password: "",
			wantErr:  ErrInvalidCredentials,
		},
		{
			// A database outage must NOT masquerade as a bad password: that
			// would tell the user their credentials are wrong and hide the
			// outage from whoever is on call.
			name:     "repository failure is not a credential error",
			finder:   fakeUserFinder{err: errors.New("connection refused")},
			password: "correct-horse",
			wantErr:  nil, // asserted separately below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewAuthService(tt.finder, tt.issuer)

			token, err := svc.Login(context.Background(), "bob@example.com", tt.password)

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				if token != "" {
					t.Errorf("got token %q on a failed login, want empty", token)
				}

			case tt.name == "repository failure is not a credential error":
				if err == nil {
					t.Fatal("got nil error for a repository failure")
				}
				if errors.Is(err, domain.ErrUnauthorized) {
					t.Error("a database failure was reported as an auth failure")
				}

			default:
				if err != nil {
					t.Fatalf("got unexpected error: %v", err)
				}
				if token != tt.wantToken {
					t.Errorf("got token %q, want %q", token, tt.wantToken)
				}
			}
		})
	}
}

// TestLoginFailuresAreIndistinguishable is the account-enumeration guard.
//
// It is a separate test rather than a table row because the property under test
// is a comparison BETWEEN two cases, not a fact about either one alone.
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	user := testUser(t, "correct-horse")

	unknownEmail := NewAuthService(
		fakeUserFinder{err: domain.NotFoundf("user not found")}, fakeTokenIssuer{})
	wrongPassword := NewAuthService(
		fakeUserFinder{user: user}, fakeTokenIssuer{})

	_, errUnknown := unknownEmail.Login(context.Background(), "nobody@example.com", "whatever")
	_, errWrong := wrongPassword.Login(context.Background(), "bob@example.com", "wrong")

	if errUnknown == nil || errWrong == nil {
		t.Fatal("both logins must fail")
	}

	// Identical text: handler.HandleError puts err.Error() straight into the
	// response body, so differing messages would mean differing 401 bodies.
	if errUnknown.Error() != errWrong.Error() {
		t.Errorf("failure messages differ and leak which emails exist:\n  unknown email: %q\n  wrong password: %q",
			errUnknown.Error(), errWrong.Error())
	}

	// Identical classification: both must map to the same HTTP status.
	if !errors.Is(errUnknown, domain.ErrUnauthorized) || !errors.Is(errWrong, domain.ErrUnauthorized) {
		t.Error("both failures must map to 401")
	}

	// Same sentinel value, so there is no way for a caller to tell them apart
	// even by identity.
	if !errors.Is(errUnknown, ErrInvalidCredentials) || !errors.Is(errWrong, ErrInvalidCredentials) {
		t.Error("both failures must be ErrInvalidCredentials")
	}
}

func TestAuthServiceCurrentUser(t *testing.T) {
	user := testUser(t, "correct-horse")

	const repoFailure = "repository failure is not an auth error"

	tests := []struct {
		name    string
		finder  fakeUserFinder
		want    *model.User
		wantErr error
	}{
		{
			name:   "existing user is returned",
			finder: fakeUserFinder{user: user},
			want:   user,
		},
		{
			// A validly signed token naming a row that has since been deleted.
			// This must be 401 and not the 404 that domain.ErrNotFound would
			// produce: the credential no longer identifies anyone, and "log in
			// again" is the useful instruction.
			name:    "deleted user becomes a stale session",
			finder:  fakeUserFinder{err: domain.NotFoundf("user not found")},
			wantErr: ErrStaleSession,
		},
		{
			// Same reasoning as the login path: an outage must not be reported
			// as an expired session, or the user re-logs in forever while the
			// real fault stays invisible.
			name:    repoFailure,
			finder:  fakeUserFinder{err: errors.New("connection refused")},
			wantErr: nil, // asserted separately below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewAuthService(tt.finder, fakeTokenIssuer{})

			got, err := svc.CurrentUser(context.Background(), uuid.New())

			switch {
			case tt.name == repoFailure:
				if err == nil {
					t.Fatal("got nil error for a repository failure")
				}
				if errors.Is(err, domain.ErrUnauthorized) {
					t.Error("a database failure was reported as an auth failure")
				}

			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				// The status the handler will derive from it.
				if !errors.Is(err, domain.ErrUnauthorized) {
					t.Error("a stale session must map to 401")
				}
				if got != nil {
					t.Errorf("got user %+v on a failed lookup, want nil", got)
				}

			default:
				if err != nil {
					t.Fatalf("got unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("got user %+v, want %+v", got, tt.want)
				}
			}
		})
	}
}
