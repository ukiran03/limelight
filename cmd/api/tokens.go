package main

import (
	"errors"
	"net/http"
	"time"

	"ukiran.com/limelight/internal/data"
	"ukiran.com/limelight/internal/validator"
)

func (app *application) createAuthenticationTokenHandler(
	w http.ResponseWriter, r *http.Request,
) {
	// Parse the email and password from the request body.
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Lookup the user record based on the email address. If no matching user
	// was found, then we call the app.invalidCredentialsResponse() helper to
	// send a 401 Unauthorized response to the client.
	user, err := app.models.Users.GetByEmail(r.Context(), input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the provided password matches the actual password for the user.
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	// If the password is correct, we generate a new token with a 24-hour
	// expiry time.
	token, err := app.models.Tokens.New(
		r.Context(), user.ID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Encode the token to JSON and send it in the response
	err = app.writeJSON(
		w, http.StatusCreated, envelope{"authentication_token": token}, nil,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
