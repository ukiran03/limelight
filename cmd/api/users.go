package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"ukiran.com/limelight/internal/data"
	"ukiran.com/limelight/internal/validator"
)

func (app *application) registerUserHandler(
	w http.ResponseWriter, r *http.Request,
) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Users.Insert(r.Context(), user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")

			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// send the email in a background goroutine
	app.background(func() {
		// create a 10-second timeout specifically for this email attempt
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = app.mailer.Send(ctx, user.Email, "user_welcome.tmpl", user)
		if err != nil {
			app.logger.Error(fmt.Sprintf("failed to send email: %v", err))
		}
	})

	err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
