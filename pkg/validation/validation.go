// Package validation provides types and utilities for validating struct fields
// and collecting validation errors. It uses a builder pattern for composing
// validation rules and supports structured error reporting with field-level
// details, codes, and parameters.
package validation

import (
	"errors"
	"maps"
)

const UnknownField = "unknown"

type Error struct {
	Field   string         `json:"field"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Params  map[string]any `json:"params,omitempty"`
}

type Rule struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Params  map[string]any `json:"params,omitempty"`
}

type Rules map[string][]Rule

type ValidationErrors []Error

func (e ValidationErrors) Error() string {
	return "validation failed"
}

func (e ValidationErrors) Empty() bool {
	return len(e) == 0
}

func (e ValidationErrors) Len() int {
	return len(e)
}

func (e ValidationErrors) ToMap() map[string]string {
	result := map[string]string{}
	for _, validationErr := range e {
		if _, exists := result[validationErr.Field]; exists {
			continue
		}
		result[validationErr.Field] = validationErr.Message
	}

	return result
}

func As(err error) (ValidationErrors, bool) {
	if err == nil {
		return nil, false
	}

	if validationErrors, ok := errors.AsType[ValidationErrors](err); ok {
		return validationErrors, true
	}

	return nil, false
}

// Join combines validation errors into one ValidationErrors and joins any other
// error onto the result. Nil errors are dropped.
func Join(errs ...error) error {
	var (
		validationErrors ValidationErrors
		otherErrors      []error
	)

	var collect func(error)
	collect = func(err error) {
		if err == nil {
			return
		}

		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			for _, joinedErr := range joined.Unwrap() {
				collect(joinedErr)
			}

			return
		}

		if wrapped, ok := err.(interface{ Unwrap() error }); ok {
			collect(wrapped.Unwrap())

			return
		}

		if validationErrs, ok := errors.AsType[ValidationErrors](err); ok {
			validationErrors = append(validationErrors, validationErrs...)
		}
	}

	for _, err := range errs {
		collect(err)
		// Keep wrappers intact so their context and errors.Is/As behaviour survive.
		if _, ok := err.(ValidationErrors); err != nil && !ok {
			otherErrors = append(otherErrors, err)
		}
	}

	if len(validationErrors) == 0 {
		return errors.Join(otherErrors...)
	}

	if len(otherErrors) == 0 {
		return validationErrors
	}

	return errors.Join(validationErrors, errors.Join(otherErrors...))
}

// Field builds a ValidationErrors holding one field error.
func Field(field, code, message string) error {
	b := NewBuilder()
	b.Add(field, code, message)

	return b.Err()
}

type ValidationBuilder struct {
	errors ValidationErrors
	rules  Rules
}

func NewBuilder() *ValidationBuilder {
	return &ValidationBuilder{
		rules: Rules{},
	}
}

func (b *ValidationBuilder) Add(field, code, message string) {
	b.errors = append(b.errors, Error{
		Field:   normalizeField(field),
		Code:    code,
		Message: message,
	})
}

func (b *ValidationBuilder) AddWithParams(
	field string,
	code string,
	message string,
	params map[string]any,
) {
	b.errors = append(b.errors, Error{
		Field:   normalizeField(field),
		Code:    code,
		Message: message,
		Params:  params,
	})
}

func (b *ValidationBuilder) AddField(field string, code, message string) {
	b.Add(field, code, message)
}

func (b *ValidationBuilder) AddFieldWithParams(
	field string,
	code string,
	message string,
	params map[string]any,
) {
	b.AddWithParams(field, code, message, params)
}

func (b *ValidationBuilder) AddRule(field, code, message string) {
	b.AddRuleWithParams(field, code, message, nil)
}

func (b *ValidationBuilder) AddRuleWithParams(
	field string,
	code string,
	message string,
	params map[string]any,
) {
	field = normalizeField(field)
	if b.rules == nil {
		b.rules = Rules{}
	}

	b.rules[field] = append(b.rules[field], Rule{
		Code:    code,
		Message: message,
		Params:  copyRuleParams(params),
	})
}

func (b *ValidationBuilder) Err() error {
	if len(b.errors) == 0 {
		return nil
	}

	return b.errors
}

func (b *ValidationBuilder) Errors() ValidationErrors {
	return b.errors
}

func (b *ValidationBuilder) Rules() Rules {
	rules := make(Rules, len(b.rules))
	for field, fieldRules := range b.rules {
		fieldRulesCopy := make([]Rule, len(fieldRules))
		for i, rule := range fieldRules {
			fieldRulesCopy[i] = Rule{
				Code:    rule.Code,
				Message: rule.Message,
				Params:  copyRuleParams(rule.Params),
			}
		}
		rules[field] = fieldRulesCopy
	}

	return rules
}

func copyRuleParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}

	paramsCopy := make(map[string]any, len(params))
	maps.Copy(paramsCopy, params)

	return paramsCopy
}

func normalizeField(field string) string {
	if field == "" {
		return UnknownField
	}

	return field
}
