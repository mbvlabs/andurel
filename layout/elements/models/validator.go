package models

import (
	"github.com/go-playground/validator/v10"
)

var validate = setupValidator()

func setupValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	return v
}
