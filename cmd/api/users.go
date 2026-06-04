package main

import (
	"errors"
	"net/http"
	"time"

	"ukiran.com/limelight/internal/data"
	"ukiran.com/limelight/internal/validator"
	"ukiran.com/limelight/internal/workers"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()
	tx, err := app.dbPool.Begin(ctx)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	defer tx.Rollback(ctx)

	txModels := data.NewModels(app.models.Movies, tx)

	err = txModels.Users.Insert(ctx, user)
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

	// Add the "movies:read" permission for the new user
	err = txModels.Permissions.AddForUser(ctx, user.ID, "movies:read")
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// After the user record has been created in the database, generate a new
	// activation token for the user
	token, err := txModels.Tokens.New(
		ctx, user.ID, (3 * 24 * time.Hour), data.ScopeActivation,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Pass the same raw 'tx' directly into River
	_, err = app.riverClient.InsertTx(ctx, tx,
		workers.OnBoardEmailArgs{
			Email:             user.Email,
			EmailTemplateFile: "user_welcome.tmpl",
			EmailData: map[string]any{
				"activationToken": token.Plaintext,
				"userID":          user.ID,
			},
		},
		nil,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Commit the transaction, Everything hits the database successfully!
	if err := tx.Commit(ctx); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) activateUserHandler(
	w http.ResponseWriter, r *http.Request,
) {
	// Parse the plaintext activation token from the request body.
	var input struct {
		TokenPlaintext string `json:"token"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	// Validate the plaintext token provided by the client.
	v := validator.New()

	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Retrieve the details of the user associated with the token using the
	// GetForToken() method. If no matching record is found, then we let the
	// client know that the token they provided is not valid.
	user, err := app.models.Users.GetForToken(
		r.Context(), data.ScopeActivation, input.TokenPlaintext,
	)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Update the user's activation status.
	user.Activated = true

	// Save the updated user record in our database, checking for any edit
	// conflicts in the same way that we did for our movie records.
	err = app.models.Users.Update(r.Context(), user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Tokens.DeleteAllForUser(
		r.Context(), data.ScopeActivation, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
