// Package hypermedia provides hypermedia SSE protocol support and helpers.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package hypermedia

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
)

const (
	DatastarKey = "datastar"
	// The default duration for retrying SSE on connection reset. This is part of the underlying retry mechanism of SSE.
	DefaultSseRetryDuration           = 1000 * time.Millisecond
	DefaultElementsUseViewTransitions = false
	DefaultPatchSignalsOnlyIfMissing  = false
	SelectorDatalineLiteral           = "selector "
	ModeDatalineLiteral               = "mode "
	ElementsDatalineLiteral           = "elements "
	UseViewTransitionDatalineLiteral  = "useViewTransition "
	SignalsDatalineLiteral            = "signals "
	OnlyIfMissingDatalineLiteral      = "onlyIfMissing "
)

type executeScriptOptions struct {
	EventID       string
	AutoRemove    *bool
	Attributes    []string
	RetryDuration time.Duration
}

type ExecuteScriptOption func(*executeScriptOptions)

func WithExecuteScriptEventID(id string) ExecuteScriptOption {
	return func(o *executeScriptOptions) {
		o.EventID = id
	}
}

func WithExecuteScriptRetryDuration(retryDuration time.Duration) ExecuteScriptOption {
	return func(o *executeScriptOptions) {
		o.RetryDuration = retryDuration
	}
}

func WithExecuteScriptAutoRemove(autoremove bool) ExecuteScriptOption {
	return func(o *executeScriptOptions) {
		o.AutoRemove = &autoremove
	}
}

func WithExecuteScriptAttributes(attributes ...string) ExecuteScriptOption {
	return func(o *executeScriptOptions) {
		o.Attributes = attributes
	}
}

func WithExecuteScriptAttributeKVs(kvs ...string) ExecuteScriptOption {
	if len(kvs)%2 != 0 {
		panic("WithExecuteScriptAttributeKVs requires an even number of arguments")
	}
	attributes := make([]string, 0, len(kvs)/2)
	for i := 0; i < len(kvs); i += 2 {
		attribute := fmt.Sprintf(`%s="%s"`, kvs[i], kvs[i+1])
		attributes = append(attributes, attribute)
	}
	return WithExecuteScriptAttributes(attributes...)
}

type ElementPatchMode string

const (
	// Default value for ElementPatchMode
	// Morphs the element into the existing element.
	DefaultElementPatchMode = ElementPatchModeOuter

	// Morphs the element into the existing element.
	ElementPatchModeOuter ElementPatchMode = "outer"
	// Replaces the inner HTML of the existing element.
	ElementPatchModeInner ElementPatchMode = "inner"
	// Removes the existing element.
	ElementPatchModeRemove ElementPatchMode = "remove"
	// Replaces the existing element with the new element.
	ElementPatchModeReplace ElementPatchMode = "replace"
	// Prepends the element inside to the existing element.
	ElementPatchModePrepend ElementPatchMode = "prepend"
	// Appends the element inside the existing element.
	ElementPatchModeAppend ElementPatchMode = "append"
	// Inserts the element before the existing element.
	ElementPatchModeBefore ElementPatchMode = "before"
	// Inserts the element after the existing element.
	ElementPatchModeAfter ElementPatchMode = "after"
)

type patchElementOptions struct {
	EventID            string
	RetryDuration      time.Duration
	Selector           string
	Mode               ElementPatchMode
	UseViewTransitions bool
}

type PatchElementOption func(*patchElementOptions)

func WithPatchElementsEventID(id string) PatchElementOption {
	return func(o *patchElementOptions) {
		o.EventID = id
	}
}

func WithSelectorf(selectorFormat string, args ...any) PatchElementOption {
	selector := fmt.Sprintf(selectorFormat, args...)
	return WithSelector(selector)
}

func WithSelector(selector string) PatchElementOption {
	return func(o *patchElementOptions) {
		o.Selector = selector
	}
}

func WithMode(merge ElementPatchMode) PatchElementOption {
	return func(o *patchElementOptions) {
		o.Mode = merge
	}
}

func WithModeOuter() PatchElementOption {
	return WithMode(ElementPatchModeOuter)
}

func WithModeInner() PatchElementOption {
	return WithMode(ElementPatchModeInner)
}

func WithModeRemove() PatchElementOption {
	return WithMode(ElementPatchModeRemove)
}

func WithModeReplace() PatchElementOption {
	return WithMode(ElementPatchModeReplace)
}

func WithModePrepend() PatchElementOption {
	return WithMode(ElementPatchModePrepend)
}

func WithModeAppend() PatchElementOption {
	return WithMode(ElementPatchModeAppend)
}

func WithModeBefore() PatchElementOption {
	return WithMode(ElementPatchModeBefore)
}

func WithModeAfter() PatchElementOption {
	return WithMode(ElementPatchModeAfter)
}

func WithSelectorID(id string) PatchElementOption {
	return WithSelector("#" + id)
}

func WithViewTransitions() PatchElementOption {
	return WithUseViewTransitions(true)
}

func WithoutViewTransitions() PatchElementOption {
	return WithUseViewTransitions(false)
}

func WithUseViewTransitions(useViewTransition bool) PatchElementOption {
	return func(o *patchElementOptions) {
		o.UseViewTransitions = useViewTransition
	}
}

func WithRetryDuration(retryDuration time.Duration) PatchElementOption {
	return func(o *patchElementOptions) {
		o.RetryDuration = retryDuration
	}
}

func ExecuteScript(c *echo.Context, scriptContents string, opts ...ExecuteScriptOption) error {
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Content-Type", "text/event-stream")

	rc := http.NewResponseController(c.Response())
	if err := rc.Flush(); err != nil {
		return fmt.Errorf("failed to flush headers: %w", err)
	}

	options := &executeScriptOptions{
		RetryDuration: DefaultSseRetryDuration,
		Attributes:    []string{},
	}
	for _, opt := range opts {
		opt(options)
	}

	sb := strings.Builder{}
	sb.WriteString("<script")

	for _, attribute := range options.Attributes {
		sb.WriteString(" ")
		sb.WriteString(attribute)
	}

	if options.AutoRemove == nil || *options.AutoRemove {
		sb.WriteString(` data-effect="el.remove()"`)
	}

	sb.WriteString(">")
	sb.WriteString(scriptContents)
	sb.WriteString("</script>")

	sendOptions := make([]SSEEventOption, 0, 2)
	if options.EventID != "" {
		sendOptions = append(sendOptions, WithSSEEventID(options.EventID))
	}
	if options.RetryDuration > 0 {
		sendOptions = append(sendOptions, WithSSERetryDuration(options.RetryDuration))
	}

	dataRows := make([]string, 0)
	dataRows = append(dataRows, SelectorDatalineLiteral+"body")
	dataRows = append(dataRows, ModeDatalineLiteral+string(ElementPatchModeAppend))

	parts := strings.SplitSeq(sb.String(), "\n")
	for part := range parts {
		dataRows = append(dataRows, ElementsDatalineLiteral+part)
	}

	if err := Send(
		c,
		EventTypePatchElements,
		dataRows,
		sendOptions...,
	); err != nil {
		return fmt.Errorf("failed to execute script: %w", err)
	}

	return nil
}

//	func ConsoleLog(c *echo.Context, msg string, opts ...ExecuteScriptOption) error {
//		call := fmt.Sprintf("console.log(%q)", msg)
//		return ExecuteScript(c, call, opts...)
//	}
//
//	func ConsoleLogf(c *echo.Context, format string, args ...any) error {
//		return ConsoleLog(c, fmt.Sprintf(format, args...))
//	}
//
//	func ConsoleError(c *echo.Context, err error, opts ...ExecuteScriptOption) error {
//		call := fmt.Sprintf("console.error(%q)", err.Error())
//		return ExecuteScript(c, call, opts...)
//	}
func Redirectf(c *echo.Context, format string, args ...any) error {
	url := fmt.Sprintf(format, args...)
	return Redirect(c, url)
}

func Redirect(c *echo.Context, url string, opts ...ExecuteScriptOption) error {
	js := fmt.Sprintf("setTimeout(() => window.location.href = %q)", url)
	return ExecuteScript(c, js, opts...)
}

func ReplaceURL(c *echo.Context, u url.URL, opts ...ExecuteScriptOption) error {
	js := fmt.Sprintf(`window.history.replaceState({}, "", %q)`, u.String())
	return ExecuteScript(c, js, opts...)
}

func ReplaceURLQuery(
	c *echo.Context,
	r *http.Request,
	values url.Values,
	opts ...ExecuteScriptOption,
) error {
	u := *r.URL
	u.RawQuery = values.Encode()
	return ReplaceURL(c, u, opts...)
}

func Prefetch(c *echo.Context, urls ...string) error {
	wrappedURLs := make([]string, len(urls))
	for i, url := range urls {
		wrappedURLs[i] = fmt.Sprintf(`"%s"`, url)
	}
	script := fmt.Sprintf(`
{
    "prefetch": [
        {
            "source": "list",
            "urls": [
                %s
            ]
        }
    ]
}
        `, strings.Join(wrappedURLs, ",\n\t\t\t\t"))
	return ExecuteScript(
		c,
		script,
		WithExecuteScriptAutoRemove(false),
		WithExecuteScriptAttributes(`type="speculationrules"`),
	)
}

type EventType string

const (
	// An event for patching HTML elements into the DOM.
	EventTypePatchElements EventType = "datastar-patch-elements"
	// An event for patching signals.
	EventTypePatchSignals EventType = "datastar-patch-signals"
	// An event for executing scripts.
	EventTypeExecuteScript EventType = "datastar-execute-script"
	// An event for merging signals.
	EventTypeMergeSignals EventType = "datastar-merge-signals"
)

const (
	NewLine       = "\n"
	DoubleNewLine = "\n\n"
)

var (
	newLineBuf       = []byte(NewLine)
	doubleNewLineBuf = []byte(DoubleNewLine)
)

// serverSentEventData holds event configuration data for
// [SSEEventOption]s.
type serverSentEventData struct {
	Type          EventType
	EventID       string
	Data          []string
	RetryDuration time.Duration
}

// SSEEventOption modifies one server-sent event.
type SSEEventOption func(*serverSentEventData)

// WithSSEEventID configures an optional event ID for one server-sent event.
// The client message field [lastEventId] will be set to this value.
// If the next event does not have an event ID, the last used event ID will remain.
//
// [lastEventId]: https://developer.mozilla.org/en-US/docs/Web/API/MessageEvent/lastEventId
func WithSSEEventID(id string) SSEEventOption {
	return func(e *serverSentEventData) {
		e.EventID = id
	}
}

// WithSSERetryDuration overrides the [DefaultSseRetryDuration] for
// one server-sent event.
func WithSSERetryDuration(retryDuration time.Duration) SSEEventOption {
	return func(e *serverSentEventData) {
		e.RetryDuration = retryDuration
	}
}

var (
	eventLinePrefix = []byte("event: ")
	idLinePrefix    = []byte("id: ")
	retryLinePrefix = []byte("retry: ")
	dataLinePrefix  = []byte("data: ")
)

func writeJustError(w io.Writer, b []byte) (err error) {
	_, err = w.Write(b)
	return err
}
