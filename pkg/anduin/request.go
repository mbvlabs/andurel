package anduin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// Header returns the first value of the named request header.
func Header(ctx context.Context, key string) string {
	if request := Request(ctx); request != nil {
		return request.Header.Get(key)
	}
	return ""
}

// Query returns the first query parameter value for key.
func Query(ctx context.Context, key string) string {
	if request := Request(ctx); request != nil {
		return request.URL.Query().Get(key)
	}
	return ""
}

// QueryValues returns every value for the named query parameter.
func QueryValues(ctx context.Context, key string) []string {
	if request := Request(ctx); request != nil {
		return request.URL.Query()[key]
	}
	return nil
}

// QueryInt parses an integer query parameter. It returns def when the parameter
// is absent and an error when it is present but not an integer.
func QueryInt(ctx context.Context, key string, def int) (int, error) {
	raw := strings.TrimSpace(Query(ctx, key))
	if raw == "" {
		return def, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return def, &HTTPError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("query parameter %q must be an integer", key),
			Err:     err,
		}
	}
	return value, nil
}

// QueryBool parses a boolean query parameter. It returns def when absent.
func QueryBool(ctx context.Context, key string, def bool) (bool, error) {
	raw := strings.TrimSpace(Query(ctx, key))
	if raw == "" {
		return def, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return def, &HTTPError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("query parameter %q must be a boolean", key),
			Err:     err,
		}
	}
	return value, nil
}

// FormValue returns the first value of the named form field.
func FormValue(ctx context.Context, key string) string {
	request := Request(ctx)
	if request == nil {
		return ""
	}
	return request.FormValue(key)
}

// Cookie returns the named request cookie, or http.ErrNoCookie.
func Cookie(ctx context.Context, name string) (*http.Cookie, error) {
	request := Request(ctx)
	if request == nil {
		return nil, http.ErrNoCookie
	}
	return request.Cookie(name)
}

// DecodeJSON decodes the request body as JSON into dst.
func DecodeJSON(ctx context.Context, dst any) error {
	request := Request(ctx)
	if request == nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "anduin: no request on context"}
	}
	if request.Body == nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: "anduin: missing request body"}
	}

	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(dst); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: "anduin: invalid JSON body", Err: err}
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &HTTPError{Status: http.StatusBadRequest, Message: "anduin: unexpected trailing JSON"}
	}

	return nil
}

// DecodeForm decodes an application/x-www-form-urlencoded (or multipart) body
// into dst using `form` struct tags, falling back to the field name.
func DecodeForm(ctx context.Context, dst any) error {
	request := Request(ctx)
	if request == nil {
		return &HTTPError{Status: http.StatusInternalServerError, Message: "anduin: no request on context"}
	}
	if err := request.ParseForm(); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: "anduin: invalid form body", Err: err}
	}

	if err := bindFormValues(request.PostForm, dst); err != nil {
		return &HTTPError{Status: http.StatusBadRequest, Message: "anduin: invalid form body", Err: err}
	}

	return nil
}

func bindFormValues(values map[string][]string, dst any) error {
	if dst == nil {
		return fmt.Errorf("anduin: decode form: nil destination")
	}

	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("anduin: decode form: destination must be a non-nil pointer")
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return fmt.Errorf("anduin: decode form: destination must point to a struct")
	}

	valueType := value.Type()
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		if !field.IsExported() {
			continue
		}
		name := strings.TrimSpace(field.Tag.Get("form"))
		if name == "" {
			name = field.Name
		}
		if idx := strings.IndexByte(name, ','); idx != -1 {
			name = name[:idx]
		}
		if name == "-" {
			continue
		}

		raw, ok := values[name]
		if !ok || len(raw) == 0 {
			continue
		}
		if err := setFormField(value.Field(i), raw[0]); err != nil {
			return fmt.Errorf("field %s: %w", field.Name, err)
		}
	}

	return nil
}

func setFormField(field reflect.Value, raw string) error {
	if !field.CanSet() {
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		field.SetFloat(parsed)
	default:
		return fmt.Errorf("unsupported field kind %s", field.Kind())
	}

	return nil
}
