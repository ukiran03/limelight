package main

import (
	"fmt"
	"net/http"
)

// logError method is helper for logging an error message
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
	w http.ResponseWriter, r *http.Request, status int, message any,
) {
	env := envelope{"error": message}

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
	message := "server encountered a problem and could not process your request"
	app.errorResponse(w, r, http.StatusInternalServerError, message)
}

// notFoundResponse method will be used to send a 404 Not Found and JSON
// response to client
func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the request resource could not be found"
	app.errorResponse(w, r, http.StatusNotFound, message)
}

// methodNotAllowedResponse method will be used to send a 405 Method Not
// Allowed status code and JSON response to the client.
func (app *application) methodNotAllowedResponse(
	w http.ResponseWriter, r *http.Request,
) {
	message := fmt.Sprintf(
		"the %s method is not supported for this resource", r.Method)
	app.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}
