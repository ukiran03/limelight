package main

import (
	"fmt"
	"net/http"
)

// logError method is helper for logging an error msg
func (app *application) logError(r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	app.logger.Error(err.Error(), "method", method, "uri", uri)
}

// errorResponse method is helper for sending JSON-formatted errr messages to
// the client
func (app *application) errorResponse(
	w http.ResponseWriter, r *http.Request, status int, msg any,
) {
	env := envelope{"error": msg}

	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(500)
	}
}

func (app *application) badRequestResponse(
	w http.ResponseWriter, r *http.Request, err error,
) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

// severErrorResponse method is for when our application encounters an
// unexpected problem at runtime
func (app *application) serverErrorResponse(
	w http.ResponseWriter, r *http.Request, err error,
) {
	app.logError(r, err)
	msg := "server encountered a problem and could not process your request"
	app.errorResponse(w, r, http.StatusInternalServerError, msg)
}

// notFoundResponse method will be used to send a 404 Not Found and JSON
// response to client
func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	msg := "the request resource could not be found"
	app.errorResponse(w, r, http.StatusNotFound, msg)
}

// methodNotAllowedResponse method will be used to send a 405 Method Not
// Allowed status code and JSON response to the client.
func (app *application) methodNotAllowedResponse(
	w http.ResponseWriter, r *http.Request,
) {
	msg := fmt.Sprintf(
		"the %s method is not supported for this resource", r.Method)
	app.errorResponse(w, r, http.StatusMethodNotAllowed, msg)
}

func (app *application) failedValidationResponse(
	w http.ResponseWriter, r *http.Request, errors map[string]string,
) {
	app.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

func (app *application) editConflictResponse(
	w http.ResponseWriter, r *http.Request,
) {
	msg := `unable to update the record due to an edit conflict,please try again`
	app.errorResponse(w, r, http.StatusConflict, msg)
}

func (app *application) rateLimitExceededResponse(
	w http.ResponseWriter, r *http.Request,
) {
	msg := "rate limit exceeded"
	app.errorResponse(w, r, http.StatusTooManyRequests, msg)
}

func (app *application) invalidCredentialsResponse(
	w http.ResponseWriter, r *http.Request,
) {
	message := "invalid authentication credentials"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) invalidAuthenticationTokenResponse(
	w http.ResponseWriter, r *http.Request,
) {
	w.Header().Set("WWW-Authenticate", "Bearer")

	message := "invalid or missing authentication token"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}
