// Package hypermedia provides hypermedia SSE protocol support and helpers.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package hypermedia

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
	"github.com/valyala/bytebufferpool"
)

type Broadcaster struct {
	ctx             context.Context
	mu              *sync.Mutex
	w               io.Writer
	rc              *http.ResponseController
	shouldLogPanics bool
	encoding        string
	acceptEncoding  string
}

func NewBroadcaster(c *echo.Context) (*Broadcaster, error) {
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Content-Type", "text/event-stream")

	if c.Request().ProtoMajor == 1 {
		c.Response().Header().Set("Connection", "keep-alive")
	}

	rc := http.NewResponseController(c.Response())

	if err := rc.Flush(); err != nil {
		return nil, fmt.Errorf("failed to flush headers: %w", err)
	}

	return &Broadcaster{
		ctx:             c.Request().Context(),
		mu:              &sync.Mutex{},
		w:               c.Response(),
		rc:              rc,
		shouldLogPanics: true,
		acceptEncoding:  c.Request().Header.Get("Accept-Encoding"),
	}, nil
}

func (sse *Broadcaster) IsClosed() bool {
	return sse.ctx.Err() != nil
}

func (sse *Broadcaster) PatchElements(elements string, opts ...PatchElementOption) error {
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

	if err := sse.Send(
		EventTypePatchElements,
		dataRows,
		sendOptions...,
	); err != nil {
		return fmt.Errorf("failed to send elements: %w", err)
	}

	return nil
}

func (sse *Broadcaster) PatchElementf(format string, args ...any) error {
	elements := fmt.Sprintf(format, args...)
	return sse.PatchElements(elements)
}

func (sse *Broadcaster) RemoveElement(selector string, opts ...PatchElementOption) error {
	opts = append(opts, WithSelector(selector), WithModeRemove())
	return sse.PatchElements("", opts...)
}

func (sse *Broadcaster) RemoveElementf(selectorFormat string, args ...any) error {
	selector := fmt.Sprintf(selectorFormat, args...)
	return sse.RemoveElement(selector)
}

func (sse *Broadcaster) RemoveElementByID(id string, opts ...PatchElementOption) error {
	return sse.RemoveElement("#"+id, opts...)
}

func (sse *Broadcaster) PatchElementTempl(comp templ.Component, opts ...PatchElementOption) error {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	if err := comp.Render(sse.ctx, buf); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	if err := sse.PatchElements(buf.String(), opts...); err != nil {
		return fmt.Errorf("failed to patch element: %w", err)
	}

	return nil
}

func (sse *Broadcaster) ExecuteScript(scriptContents string, opts ...ExecuteScriptOption) error {
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

	if err := sse.Send(
		EventTypePatchElements,
		dataRows,
		sendOptions...,
	); err != nil {
		return fmt.Errorf("failed to execute script: %w", err)
	}

	return nil
}

func (sse *Broadcaster) ConsoleLog(msg string, opts ...ExecuteScriptOption) error {
	call := fmt.Sprintf("console.log(%q)", msg)
	return sse.ExecuteScript(call, opts...)
}

func (sse *Broadcaster) ConsoleLogf(format string, args ...any) error {
	return sse.ConsoleLog(fmt.Sprintf(format, args...))
}

func (sse *Broadcaster) ConsoleError(err error, opts ...ExecuteScriptOption) error {
	call := fmt.Sprintf("console.error(%q)", err.Error())
	return sse.ExecuteScript(call, opts...)
}

func (sse *Broadcaster) Redirectf(format string, args ...any) error {
	url := fmt.Sprintf(format, args...)
	return sse.Redirect(url)
}

func (sse *Broadcaster) Redirect(url string, opts ...ExecuteScriptOption) error {
	js := fmt.Sprintf("setTimeout(() => window.location.href = %q)", url)
	return sse.ExecuteScript(js, opts...)
}

func (sse *Broadcaster) DispatchCustomEvent(
	eventName string,
	detail any,
	opts ...DispatchCustomEventOption,
) error {
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

	return sse.ExecuteScript(js, executeOptions...)
}

func (sse *Broadcaster) ReplaceURL(u url.URL, opts ...ExecuteScriptOption) error {
	js := fmt.Sprintf(`window.history.replaceState({}, "", %q)`, u.String())
	return sse.ExecuteScript(js, opts...)
}

func (sse *Broadcaster) ReplaceURLQuery(
	r *http.Request,
	values url.Values,
	opts ...ExecuteScriptOption,
) error {
	u := *r.URL
	u.RawQuery = values.Encode()
	return sse.ReplaceURL(u, opts...)
}

func (sse *Broadcaster) Prefetch(urls ...string) error {
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
	return sse.ExecuteScript(
		script,
		WithExecuteScriptAutoRemove(false),
		WithExecuteScriptAttributes(`type="speculationrules"`),
	)
}

func (sse *Broadcaster) PatchSignals(signalsContents []byte, opts ...PatchSignalsOption) error {
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

	if err := sse.Send(
		EventTypePatchSignals,
		dataRows,
		sendOptions...,
	); err != nil {
		return fmt.Errorf("failed to send patch signals: %w", err)
	}
	return nil
}

func (sse *Broadcaster) MarshalAndPatchSignals(signals any, opts ...PatchSignalsOption) error {
	signalsJSON, err := json.Marshal(signals)
	if err != nil {
		return fmt.Errorf("failed to marshal signals: %w", err)
	}
	return sse.PatchSignals(signalsJSON, opts...)
}

func (sse *Broadcaster) MarshalAndPatchSignalsIfMissing(
	signals any,
	opts ...PatchSignalsOption,
) error {
	opts = append(opts, WithOnlyIfMissing(true))
	return sse.MarshalAndPatchSignals(signals, opts...)
}

func (sse *Broadcaster) PatchSignalsIfMissingRaw(
	signalsJSON []byte,
	opts ...PatchSignalsOption,
) error {
	opts = append(opts, WithOnlyIfMissing(true))
	return sse.PatchSignals(signalsJSON, opts...)
}

func (sse *Broadcaster) MergeSignals(signals map[string]any) error {
	dataRows := make([]string, 0, len(signals))
	for key, value := range signals {
		dataRows = append(dataRows, fmt.Sprintf("%s %v", key, value))
	}

	if err := sse.Send(
		EventTypeMergeSignals,
		dataRows,
	); err != nil {
		return fmt.Errorf("failed to merge signals: %w", err)
	}

	return nil
}

// Send emits a server-sent event to the client. Method is safe for
// concurrent use.
func (sse *Broadcaster) Send(
	eventType EventType,
	dataLines []string,
	opts ...SSEEventOption,
) error {
	if err := sse.ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	buf, err := buildSSEEvent(eventType, dataLines, opts...)
	if err != nil {
		return err
	}
	defer bytebufferpool.Put(buf)

	sse.mu.Lock()
	defer sse.mu.Unlock()

	if _, err := buf.WriteTo(sse.w); err != nil {
		return fmt.Errorf("failed to write to response writer: %w", err)
	}

	if err := sse.rc.Flush(); err != nil {
		return fmt.Errorf("failed to flush data: %w", err)
	}

	return nil
}
