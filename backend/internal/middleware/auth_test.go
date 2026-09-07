package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"ecommerce/backend/internal/handler"
	"ecommerce/backend/internal/model"
	"ecommerce/backend/internal/util"
)

func TestMain(m *testing.M) {
	// Otherwise gin writes a debug banner and a warning line per test.
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

const validToken = "valid.jwt.token"

// fakeValidator accepts exactly one token string and rejects every other.
//
// It records what it was handed, which is what lets a test assert WHICH
// credential the middleware picked rather than merely that it found one. This
// is the whole reason RequireAuth takes an interface: no signing key, no clock,
// no real HS256 token needed to test the extraction logic.
type fakeValidator struct {
	claims *util.Claims
	seen   string
}

func (f *fakeValidator) ValidateToken(raw string) (*util.Claims, error) {
	f.seen = raw
	if raw != validToken {
		return nil, util.ErrInvalidToken
	}
	return f.claims, nil
}

func testClaims(id uuid.UUID, role model.Role) *util.Claims {
	return &util.Claims{
		Role:             role,
		RegisteredClaims: jwt.RegisteredClaims{Subject: id.String()},
	}
}

// runChain wires the middleware onto a throwaway engine and returns the
// response plus the context the final handler saw. ctx is nil when the chain
// rejected the request, which is itself the assertion that it aborted.
func runChain(req *http.Request, chain ...gin.HandlerFunc) (*httptest.ResponseRecorder, *gin.Context) {
	var captured *gin.Context

	engine := gin.New()
	engine.GET("/", append(chain, func(c *gin.Context) {
		captured = c
		c.Status(http.StatusOK)
	})...)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w, captured
}

func request(cookie, authHeader string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: handler.AuthCookieName, Value: cookie})
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	return req
}

func TestRequireAuth(t *testing.T) {
	tests := []struct {
		name       string
		cookie     string
		header     string
		wantStatus int
		wantSeen   string
	}{
		{
			name:       "no credential at all",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "valid cookie is accepted",
			cookie:     validToken,
			wantStatus: http.StatusOK,
			wantSeen:   validToken,
		},
		{
			name:       "valid bearer header is accepted",
			header:     "Bearer " + validToken,
			wantStatus: http.StatusOK,
			wantSeen:   validToken,
		},
		{
			// RFC 7235 makes the scheme case-insensitive and real clients do
			// send this.
			name:       "lowercase bearer scheme is accepted",
			header:     "bearer " + validToken,
			wantStatus: http.StatusOK,
			wantSeen:   validToken,
		},
		{
			// The cookie is httpOnly and the header is not, so a header must
			// never be able to override a cookie the browser already holds.
			name:       "cookie wins when both are present",
			cookie:     validToken,
			header:     "Bearer forged.token.value",
			wantStatus: http.StatusOK,
			wantSeen:   validToken,
		},
		{
			name:       "empty cookie falls through to the header",
			cookie:     "",
			header:     "Bearer " + validToken,
			wantStatus: http.StatusOK,
			wantSeen:   validToken,
		},
		{
			name:       "rejected token",
			header:     "Bearer not.a.real.token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "header with no scheme",
			header:     validToken,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong scheme",
			header:     "Basic " + validToken,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "scheme with no token",
			header:     "Bearer ",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &fakeValidator{claims: testClaims(uuid.New(), model.RoleCustomer)}

			w, ctx := runChain(request(tt.cookie, tt.header), RequireAuth(v))

			if w.Code != tt.wantStatus {
				t.Fatalf("got status %d, want %d (body %s)", w.Code, tt.wantStatus, w.Body)
			}

			if tt.wantStatus == http.StatusOK {
				if ctx == nil {
					t.Fatal("chain aborted on a request that should have passed")
				}
				if v.seen != tt.wantSeen {
					t.Errorf("middleware validated %q, want %q", v.seen, tt.wantSeen)
				}
				return
			}

			if ctx != nil {
				t.Error("handler ran despite a rejected request")
			}
			// A 401 is supposed to tell the client how to authenticate.
			if got := w.Header().Get("WWW-Authenticate"); got != authScheme {
				t.Errorf("got WWW-Authenticate %q, want %q", got, authScheme)
			}
		})
	}
}

// TestRequireAuthPopulatesContext is the contract the handlers depend on: the
// values must arrive TYPED, because handler.UserIDFrom asserts uuid.UUID and
// would silently report "not authenticated" for a string that merely looks
// like a uuid.
func TestRequireAuthPopulatesContext(t *testing.T) {
	want := uuid.New()
	v := &fakeValidator{claims: testClaims(want, model.RoleAdmin)}

	_, ctx := runChain(request(validToken, ""), RequireAuth(v))
	if ctx == nil {
		t.Fatal("chain aborted on a valid token")
	}

	got, ok := handler.UserIDFrom(ctx)
	if !ok {
		t.Fatal("no user id on the context")
	}
	if got != want {
		t.Errorf("got user id %s, want %s", got, want)
	}

	role, ok := handler.UserRoleFrom(ctx)
	if !ok {
		t.Fatal("no role on the context")
	}
	if role != model.RoleAdmin {
		t.Errorf("got role %q, want %q", role, model.RoleAdmin)
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name       string
		role       model.Role
		allowed    []model.Role
		skipAuth   bool
		wantStatus int
	}{
		{
			name:       "matching role passes",
			role:       model.RoleAdmin,
			allowed:    []model.Role{model.RoleAdmin},
			wantStatus: http.StatusOK,
		},
		{
			name:       "wrong role is forbidden",
			role:       model.RoleCustomer,
			allowed:    []model.Role{model.RoleAdmin},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "any one of several allowed roles passes",
			role:       model.RoleCustomer,
			allowed:    []model.Role{model.RoleAdmin, model.RoleCustomer},
			wantStatus: http.StatusOK,
		},
		{
			// The mis-wiring case: RequireRole installed without RequireAuth in
			// front of it. It must fail closed, and as 401 rather than 403 —
			// there is no identity here to forbid.
			name:       "no identity on the context is 401 not 403",
			allowed:    []model.Role{model.RoleAdmin},
			skipAuth:   true,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := []gin.HandlerFunc{}
			if !tt.skipAuth {
				v := &fakeValidator{claims: testClaims(uuid.New(), tt.role)}
				chain = append(chain, RequireAuth(v))
			}
			chain = append(chain, RequireRole(tt.allowed...))

			w, ctx := runChain(request(validToken, ""), chain...)

			if w.Code != tt.wantStatus {
				t.Fatalf("got status %d, want %d (body %s)", w.Code, tt.wantStatus, w.Body)
			}
			if tt.wantStatus != http.StatusOK && ctx != nil {
				t.Error("handler ran despite a rejected request")
			}
		})
	}
}
