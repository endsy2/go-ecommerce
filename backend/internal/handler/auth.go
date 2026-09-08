package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ecommerce/backend/internal/dto"
	"ecommerce/backend/internal/model"
)

// AuthCookieName is the cookie the browser sends back on every request. Also the
// name the auth middleware will read when it is built.
const AuthCookieName = "auth_token"

// authService is the slice of AuthService this handler uses — declared by the
// consumer, as in the service layer.
type authService interface {
	Register(ctx context.Context, email, name, password string) (*model.User, string, error)
	Login(ctx context.Context, email, password string) (string, error)
	CurrentUser(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type AuthHandler struct {
	auth authService

	// cookieTTL matches the token's own expiry so the cookie cannot outlive the
	// credential it carries. A cookie that survives its token just produces
	// confusing 401s on a session the browser still believes in.
	cookieTTL time.Duration

	// secureCookie marks the cookie Secure, so the browser only ever sends it
	// over HTTPS. It has to be off in local development because that runs on
	// plain http://localhost, where a Secure cookie would simply never be sent
	// and login would appear to succeed while every later request was anonymous.
	secureCookie bool
}

func NewAuthHandler(auth authService, cookieTTL time.Duration, secureCookie bool) *AuthHandler {
	return &AuthHandler{auth: auth, cookieTTL: cookieTTL, secureCookie: secureCookie}
}

// Login answers POST /api/v1/auth/login.
//
// On success it sets the token as an httpOnly cookie AND returns it in the body.
// On failure it returns the same 401 whether the email is unknown or the
// password is wrong — see service.ErrInvalidCredentials.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleBindingError(c, err)
		return
	}

	token, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		// No branching on the failure reason. HandleError maps the single
		// ErrInvalidCredentials value to one 401 body, and a database failure
		// to a 500, without this handler needing to tell them apart.
		HandleError(c, err)
		return
	}

	h.setAuthCookie(c, token)

	OK(c, dto.LoginResponse{Token: token})
}

// Register answers POST /api/v1/auth/register.
//
// It creates the account and signs it in with the same cookie Login sets, so a
// new user is authenticated the moment they sign up. 201 rather than 200,
// because this request created a resource.
//
// An address that already exists produces a 409 rather than a generic failure.
// Unlike Login, which hides whether an email is registered, signup cannot: the
// user has to be told why their chosen address was refused. Hiding it here
// would require an email-verification flow that this project does not have.
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Field-level 422 from the binding tags — including the password policy,
		// which lives on the dto and not in this handler.
		HandleBindingError(c, err)
		return
	}

	user, token, err := h.auth.Register(c.Request.Context(), req.Email, req.Name, req.Password)
	if err != nil {
		// domain.ErrConflict from a duplicate email becomes a 409 here without
		// this handler naming that case: HandleError derives it from the error.
		HandleError(c, err)
		return
	}

	h.setAuthCookie(c, token)

	Created(c, dto.RegisterResponse{
		Token: token,
		User:  newUserResponse(user),
	})
}

// setAuthCookie writes the session cookie.
//
// Shared by Login and Register rather than written out at each call site: the
// flags below are the security properties of the session, and two copies of
// them are two things that can drift apart. A Register that forgot httpOnly
// would hand every XSS bug a token, and nothing would fail visibly.
func (h *AuthHandler) setAuthCookie(c *gin.Context, token string) {
	// SameSite=Lax stops the browser attaching this cookie to cross-site POSTs,
	// which is the cheap half of CSRF protection. Strict would be safer still,
	// but it also drops the cookie when a user arrives by following a link from
	// another site, which logs them out for no reason they can see.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		AuthCookieName,
		token,
		int(h.cookieTTL.Seconds()),
		"/",
		"", // host-only: the browser scopes it to the API's own domain
		h.secureCookie,
		true, // httpOnly: unreadable from JavaScript, so XSS cannot steal it
	)
}

// Me answers GET /api/v1/auth/me, returning the caller's own account.
//
// The route sits inside the RequireAuth group, so by the time this runs the
// identity is already on the context — this handler does no token parsing of
// its own, and could not accidentally disagree with the middleware about who
// the caller is.
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := UserIDFrom(c)
	if !ok {
		// Unreachable while this route stays in the protected group; handled
		// because the alternative to failing here is querying with uuid.Nil.
		RespondError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	user, err := h.auth.CurrentUser(c.Request.Context(), userID)
	if err != nil {
		HandleError(c, err)
		return
	}

	OK(c, newUserResponse(user))
}

// newUserResponse maps a model to its wire shape.
//
// The mapping lives in the handler layer rather than as a constructor in dto,
// so that dto never imports model and a column rename cannot quietly become an
// API change.
func newUserResponse(u *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:    u.ID.String(),
		Email: u.Email,
		Name:  u.Name,
		Role:  string(u.Role),
		// PasswordHash is deliberately absent, and UserResponse has nowhere to
		// put it.
		CreatedAt: u.CreatedAt,
	}
}
