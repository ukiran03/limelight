package data

import "ukiran.com/limelight/internal/validator"

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX),
		"email", "must be a valid email address")
}

func ValidatePasswordPlaintext(v *validator.Validator, passwd string) {
	v.Check(passwd != "", "password", "must be provided")
	v.Check(len(passwd) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(passwd) <= 72, "password", "must not be more than 72 bytes long")
}

func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")

	ValidateEmail(v, user.Email)

	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext)
	}

	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}
