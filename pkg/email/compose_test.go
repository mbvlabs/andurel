package email

import (
	"errors"
	"reflect"
	"testing"
)

type stubTransformer struct {
	html    string
	text    string
	htmlErr error
	textErr error
}

func (s stubTransformer) ToHTML() (string, error) {
	return s.html, s.htmlErr
}

func (s stubTransformer) ToText() (string, error) {
	return s.text, s.textErr
}

func TestComposeRendersBothBodies(t *testing.T) {
	data, err := Compose(
		stubTransformer{html: "<p>Hello</p>", text: "Hello"},
		"person@example.com",
		"noreply@example.com",
		"Hello",
	)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	want := TransactionalData{
		To:       "person@example.com",
		From:     "noreply@example.com",
		Subject:  "Hello",
		HTMLBody: "<p>Hello</p>",
		TextBody: "Hello",
	}
	if !reflect.DeepEqual(data, want) {
		t.Fatalf("data = %#v, want %#v", data, want)
	}
}

func TestComposeSurfacesRenderErrors(t *testing.T) {
	htmlErr := errors.New("html failed")
	if _, err := Compose(
		stubTransformer{htmlErr: htmlErr},
		"person@example.com",
		"noreply@example.com",
		"Hello",
	); !errors.Is(err, htmlErr) {
		t.Fatalf("Compose error = %v, want %v", err, htmlErr)
	}

	textErr := errors.New("text failed")
	if _, err := Compose(
		stubTransformer{textErr: textErr},
		"person@example.com",
		"noreply@example.com",
		"Hello",
	); !errors.Is(err, textErr) {
		t.Fatalf("Compose error = %v, want %v", err, textErr)
	}
}
