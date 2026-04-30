package main

import (
	"context"
	"net/http"

	"ukiran.com/limelight/internal/data"
)

type contextKey string

// using this constant as the key for getting and setting user information in
// the request context.
const userContextKey = contextKey("user")

// The contextSetUser() method returns a new copy of the request with the
// provided User struct added to the context
func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

// The contextGetUser() retrieves the User struct from the request context
func (app *application) contextGetUser(r *http.Request) *data.User {
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		panic("missing user value in request context")
	}
	return user
}
