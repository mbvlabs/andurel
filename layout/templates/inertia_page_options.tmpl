package inertia

import (
	gonertia "github.com/romsar/gonertia/v3"
)

type PageOption func(*pageOptions)

type pageOptions struct {
	validationErrors gonertia.ValidationErrors
}

func WithValidationErrors(errors map[string]string) PageOption {
	return func(opts *pageOptions) {
		if len(errors) == 0 {
			return
		}

		if opts.validationErrors == nil {
			opts.validationErrors = gonertia.ValidationErrors{}
		}
		for field, message := range errors {
			opts.validationErrors[field] = message
		}
	}
}
