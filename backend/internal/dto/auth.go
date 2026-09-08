// Package dto holds the request and response shapes for the HTTP API. These are
// the only structs with `json` tags; models never carry them, so a column rename
// cannot silently change the wire format.
package dto

import "time"

// LoginRequest is the POST /api/v1/auth/login body.
//
// Note what is NOT validated here: there is no `min=8` on Password. Password
// policy belongs on registration. Enforcing it at login would return 422 for a
// short guess and 401 for a long one, which hands an attacker a free signal
// about the policy — and worse, tells them a short candidate was not even
// checked. Every wrong password must reach the same 401.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse carries the token in the body.
//
// The same token is also set as an httpOnly cookie. The cookie is what a browser
// should use: httpOnly means JavaScript cannot read it, so an XSS bug cannot
// exfiltrate the session. The body copy exists for clients that have no cookie
// jar — a mobile app, curl, an integration test.
//
// Wrapped in a struct rather than returned as a bare string so that adding a
// field later (expires_at, a refresh token, the user profile) is not a breaking
// change to the response shape.
type LoginResponse struct {
	Token string `json:"token"`
}

// UserResponse is the GET /api/v1/auth/me body, and the shape any later
// endpoint returning a user should reuse.
//
// It is written out by hand rather than being model.User with json tags added,
// because model.User carries PasswordHash. A model that is directly
// serialisable is one careless handler away from putting a bcrypt digest on the
// wire; there is no field here that could leak one even by accident.
//
// ID and Role are strings rather than uuid.UUID and model.Role so that this
// package stays a description of the wire format alone and never imports model
// — handlers do the mapping, per backend/CLAUDE.md.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// RegisterRequest is the POST /api/v1/auth/register body.
//
// There is deliberately no Role field. Role is not something a client may ask
// for: if it were bindable here, anyone could POST `"role":"admin"` and grant
// themselves the admin group. Leaving it off the struct makes that impossible
// to express rather than something a handler has to remember to strip.
//
// Unlike LoginRequest, Password carries a policy — this is the endpoint that
// sets the password, so it is the only place a policy can be enforced.
//
// The max=72 is not a round number. bcrypt hashes the first 72 bytes of its
// input and silently discards the rest, so without a cap a 200-character
// passphrase would be truncated and the user would believe they had far more
// protection than they do. Rejecting at 72 makes the limit visible instead.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required,min=1,max=100"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// RegisterResponse is returned with 201 on a successful registration.
//
// Registering signs the new user in: the token is issued here and also set as
// the auth cookie, so the client does not have to follow a signup with an
// immediate login. The user is included alongside it to save a further round
// trip to /auth/me, and reuses UserResponse so there is exactly one wire shape
// for a user.
type RegisterResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}
