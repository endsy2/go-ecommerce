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
