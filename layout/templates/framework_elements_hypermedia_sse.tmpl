// Package hypermedia provides hypermedia SSE protocol support and helpers.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package hypermedia

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
	"github.com/valyala/bytebufferpool"
)

func PatchElements(c *echo.Context, elements string, opts ...PatchElementOption) error {
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Content-Type", "text/event-stream")

	rc := http.NewResponseController(c.Response())
	if err := rc.Flush(); err != nil {
		return fmt.Errorf("failed to flush headers: %w", err)
	}

	options := &patchElementOptions{
		EventID:       "",
		RetryDuration: DefaultSseRetryDuration,
		Selector:      "",
		Mode:          ElementPatchModeOuter,
	}
	for _, opt := range opts {
		opt(options)
	}

	sendOptions := make([]SSEEventOption, 0, 2)
	if options.EventID != "" {
		sendOptions = append(sendOptions, WithSSEEventID(options.EventID))
	}
	if options.RetryDuration > 0 {
		sendOptions = append(sendOptions, WithSSERetryDuration(options.RetryDuration))
	}

	dataRows := make([]string, 0, 4)
	if options.Selector != "" {
		dataRows = append(dataRows, SelectorDatalineLiteral+options.Selector)
	}
	if options.Mode != ElementPatchModeOuter {
		dataRows = append(dataRows, ModeDatalineLiteral+string(options.Mode))
	}
	if options.UseViewTransitions {
		dataRows = append(dataRows, UseViewTransitionDatalineLiteral+"true")
	}

	if elements != "" {
		parts := strings.SplitSeq(elements, "\n")
		for part := range parts {
			dataRows = append(dataRows, ElementsDatalineLiteral+part)
		}
	}

	if err := Send(
		c,
		EventTypePatchElements,
		dataRows,
		sendOptions...,
	); err != nil {
		return fmt.Errorf("failed to send elements: %w", err)
	}

	return nil
}

func PatchElementf(c *echo.Context, format string, args ...any) error {
	elements := fmt.Sprintf(format, args...)
	return PatchElements(c, elements)
}

// RemoveElement TODO: make clear the selector is _raw_ or consider improving inputs
func RemoveElement(c *echo.Context, selector string, opts ...PatchElementOption) error {
	opts = append(opts, WithSelector(selector), WithModeRemove())
	return PatchElements(c, "", opts...)
}

func RemoveElementf(c *echo.Context, selectorFormat string, args ...any) error {
	selector := fmt.Sprintf(selectorFormat, args...)
	return RemoveElement(c, selector)
}

func RemoveElementByID(c *echo.Context, id string, opts ...PatchElementOption) error {
	return RemoveElement(c, "#"+id, opts...)
}

func DispatchCustomEvent(
	c *echo.Context,
	eventName string,
	detail any,
	opts ...DispatchCustomEventOption,
) error {
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Content-Type", "text/event-stream")

	rc := http.NewResponseController(c.Response())
	if err := rc.Flush(); err != nil {
		return fmt.Errorf("failed to flush headers: %w", err)
	}

	if eventName == "" {
		return fmt.Errorf("eventName is required")
	}

	detailsJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("failed to marshal detail: %w", err)
	}

	const defaultSelector = "document"
	options := dispatchCustomEventOptions{
		EventID:       "",
		RetryDuration: DefaultSseRetryDuration,
		Selector:      defaultSelector,
		Bubbles:       true,
		Cancelable:    true,
		Composed:      true,
	}

	for _, opt := range opts {
		opt(&options)
	}

	elementsJS := `[document]`
	if options.Selector != "" && options.Selector != defaultSelector {
		elementsJS = fmt.Sprintf(`document.querySelectorAll(%q)`, options.Selector)
	}

	js := fmt.Sprintf(`
const elements = %s

const event = new CustomEvent(%q, {
    bubbles: %t,
    cancelable: %t,
    composed: %t,
    detail: %s,
});

elements.forEach((element) => {
    element.dispatchEvent(event);
});
    `,
		elementsJS,
		eventName,
		options.Bubbles,
		options.Cancelable,
		options.Composed,
		string(detailsJSON),
	)

	executeOptions := make([]ExecuteScriptOption, 0)
	if options.EventID != "" {
		executeOptions = append(executeOptions, WithExecuteScriptEventID(options.EventID))
	}
	if options.RetryDuration != 0 {
		executeOptions = append(
			executeOptions,
			WithExecuteScriptRetryDuration(options.RetryDuration),
		)
	}

	return ExecuteScript(c, js, executeOptions...)
}

func PatchSignals(c *echo.Context, signalsContents []byte, opts ...PatchSignalsOption) error {
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Content-Type", "text/event-stream")

	rc := http.NewResponseController(c.Response())
	if err := rc.Flush(); err != nil {
		return fmt.Errorf("failed to flush headers: %w", err)
	}

	options := &patchSignalsOptions{
		EventID:       "",
		RetryDuration: DefaultSseRetryDuration,
		OnlyIfMissing: false,
	}
	for _, opt := range opts {
		opt(options)
	}

	dataRows := make([]string, 0, 32)
	if options.OnlyIfMissing {
		dataRows = append(
			dataRows,
			OnlyIfMissingDatalineLiteral+strconv.FormatBool(options.OnlyIfMissing),
		)
	}
	lines := strings.SplitSeq(string(signalsContents), "\n")
	for line := range lines {
		dataRows = append(dataRows, SignalsDatalineLiteral+line)
	}

	sendOptions := make([]SSEEventOption, 0, 2)
	if options.EventID != "" {
		sendOptions = append(sendOptions, WithSSEEventID(options.EventID))
	}
	if options.RetryDuration != DefaultSseRetryDuration {
		sendOptions = append(sendOptions, WithSSERetryDuration(options.RetryDuration))
	}

	if err := Send(
		c,
		EventTypePatchSignals,
		dataRows,
		sendOptions...,
	); err != nil {
		return fmt.Errorf("failed to send patch signals: %w", err)
	}
	return nil
}

func MarshalAndPatchSignals(c *echo.Context, signals any, opts ...PatchSignalsOption) error {
	signalsJSON, err := json.Marshal(signals)
	if err != nil {
		return fmt.Errorf("failed to marshal signals: %w", err)
	}
	return PatchSignals(c, signalsJSON, opts...)
}

func MarshalAndPatchSignalsIfMissing(
	c *echo.Context,
	signals any,
	opts ...PatchSignalsOption,
) error {
	opts = append(opts, WithOnlyIfMissing(true))
	return MarshalAndPatchSignals(c, signals, opts...)
}

func PatchSignalsIfMissingRaw(
	c *echo.Context,
	signalsJSON []byte,
	opts ...PatchSignalsOption,
) error {
	opts = append(opts, WithOnlyIfMissing(true))
	return PatchSignals(c, signalsJSON, opts...)
}

func MergeSignals(c *echo.Context, signals map[string]any) error {
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Content-Type", "text/event-stream")

	rc := http.NewResponseController(c.Response())
	if err := rc.Flush(); err != nil {
		return fmt.Errorf("failed to flush headers: %w", err)
	}

	dataRows := make([]string, 0, len(signals))
	for key, value := range signals {
		dataRows = append(dataRows, fmt.Sprintf("%s %v", key, value))
	}

	if err := Send(
		c,
		EventTypeMergeSignals,
		dataRows,
	); err != nil {
		return fmt.Errorf("failed to merge signals: %w", err)
	}

	return nil
}

func ReadSignals(r *http.Request, signals any) error {
	var dsInput []byte

	if r.Method == "GET" {
		dsJSON := r.URL.Query().Get(DatastarKey)
		if dsJSON == "" {
			return nil
		}
		dsInput = []byte(dsJSON)
	}
	if r.Method != "GET" {
		buf := bytebufferpool.Get()
		defer bytebufferpool.Put(buf)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			if err == http.ErrBodyReadAfterClose {
				return fmt.Errorf(
					"body already closed, are you sure you created the SSE ***AFTER*** the ReadSignals? %w",
					err,
				)
			}
			return fmt.Errorf("failed to read body: %w", err)
		}
		dsInput = buf.Bytes()
	}

	if err := json.Unmarshal(dsInput, signals); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}
	return nil
}

func Send(c *echo.Context, eventType EventType, dataLines []string, opts ...SSEEventOption) error {
	if err := c.Request().Context().Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	buf, err := buildSSEEvent(eventType, dataLines, opts...)
	if err != nil {
		return err
	}
	defer bytebufferpool.Put(buf)

	if _, err := buf.WriteTo(c.Response()); err != nil {
		return fmt.Errorf("failed to write to response writer: %w", err)
	}

	rc := http.NewResponseController(c.Response())
	if err := rc.Flush(); err != nil {
		return fmt.Errorf("failed to flush data: %w", err)
	}

	return nil
}

func buildSSEEvent(
	eventType EventType,
	dataLines []string,
	opts ...SSEEventOption,
) (*bytebufferpool.ByteBuffer, error) {
	evt := serverSentEventData{
		Type:          eventType,
		Data:          dataLines,
		RetryDuration: DefaultSseRetryDuration,
	}

	for _, opt := range opts {
		opt(&evt)
	}

	buf := bytebufferpool.Get()

	if err := errors.Join(
		writeJustError(buf, eventLinePrefix),
		writeJustError(buf, []byte(evt.Type)),
		writeJustError(buf, newLineBuf),
	); err != nil {
		bytebufferpool.Put(buf)
		return nil, fmt.Errorf("failed to write event type: %w", err)
	}

	if evt.EventID != "" {
		if err := errors.Join(
			writeJustError(buf, idLinePrefix),
			writeJustError(buf, []byte(evt.EventID)),
			writeJustError(buf, newLineBuf),
		); err != nil {
			bytebufferpool.Put(buf)
			return nil, fmt.Errorf("failed to write id: %w", err)
		}
	}

	if evt.RetryDuration.Milliseconds() > 0 &&
		evt.RetryDuration.Milliseconds() != DefaultSseRetryDuration.Milliseconds() {
		retry := int(evt.RetryDuration.Milliseconds())
		retryStr := strconv.Itoa(retry)
		if err := errors.Join(
			writeJustError(buf, retryLinePrefix),
			writeJustError(buf, []byte(retryStr)),
			writeJustError(buf, newLineBuf),
		); err != nil {
			bytebufferpool.Put(buf)
			return nil, fmt.Errorf("failed to write retry: %w", err)
		}
	}

	for _, d := range evt.Data {
		if err := errors.Join(
			writeJustError(buf, dataLinePrefix),
			writeJustError(buf, []byte(d)),
			writeJustError(buf, newLineBuf),
		); err != nil {
			bytebufferpool.Put(buf)
			return nil, fmt.Errorf("failed to write data: %w", err)
		}
	}

	if err := writeJustError(buf, doubleNewLineBuf); err != nil {
		bytebufferpool.Put(buf)
		return nil, fmt.Errorf("failed to write newline: %w", err)
	}

	return buf, nil
}

func PatchElementTempl(c *echo.Context, comp templ.Component, opts ...PatchElementOption) error {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	if err := comp.Render(c.Request().Context(), buf); err != nil {
		return fmt.Errorf("failed to patch element: %w", err)
	}

	if err := PatchElements(c, buf.String(), opts...); err != nil {
		return fmt.Errorf("failed to patch element: %w", err)
	}

	return nil
}
