// Package inertia implements the Inertia v3 protocol for anduin and templ.
package inertia

import (
	"context"
	"net/http"
	"strings"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

type requestContextKey struct{}

// Request is the normalized Inertia protocol state for an HTTP request.
type Request struct {
	Inertia          bool
	Version          string
	PartialComponent string
	Only             []string
	Except           []string
	Reset            []string
	ErrorBag         string
	MergeIntent      string
	ExceptOnceProps  []string
	Purpose          string
}

// ParseRequest parses all Inertia v3 request headers from the request carried
// on ctx.
func ParseRequest(ctx context.Context) (Request, error) {
	request := anduin.Request(ctx)
	if request == nil {
		return Request{}, protocolError("parse request", "missing HTTP request")
	}

	return ParseHTTPRequest(request)
}

// ParseHTTPRequest parses all Inertia v3 request headers.
func ParseHTTPRequest(req *http.Request) (Request, error) {
	if req == nil {
		return Request{}, protocolError("parse request", "missing HTTP request")
	}

	state := Request{
		Inertia: strings.EqualFold(
			strings.TrimSpace(req.Header.Get(HeaderInertia)),
			"true",
		),
		Version:          strings.TrimSpace(req.Header.Get(HeaderVersion)),
		PartialComponent: strings.TrimSpace(req.Header.Get(HeaderPartialComponent)),
		Only:             headerList(req.Header.Get(HeaderPartialData)),
		Except:           headerList(req.Header.Get(HeaderPartialExcept)),
		Reset:            headerList(req.Header.Get(HeaderReset)),
		ErrorBag:         strings.TrimSpace(req.Header.Get(HeaderErrorBag)),
		MergeIntent: strings.ToLower(
			strings.TrimSpace(req.Header.Get(HeaderInfiniteScrollMergeIntent)),
		),
		ExceptOnceProps: headerList(req.Header.Get(HeaderExceptOnceProps)),
		Purpose:         strings.ToLower(strings.TrimSpace(req.Header.Get(HeaderPurpose))),
	}
	if state.MergeIntent != "" && state.MergeIntent != MergeIntentAppend &&
		state.MergeIntent != MergeIntentPrepend {
		return Request{}, protocolError("parse request", "invalid infinite-scroll merge intent")
	}
	return state, nil
}

// IsPartialFor reports whether partial selection applies to component.
func (r Request) IsPartialFor(component string) bool {
	return r.Inertia && r.PartialComponent != "" && r.PartialComponent == component
}

// IsPrefetch reports whether the visit is a client prefetch request.
func (r Request) IsPrefetch() bool { return r.Purpose == PurposePrefetch }

func headerList(raw string) []string {
	seen := make(map[string]struct{})
	var values []string
	for item := range strings.SplitSeq(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		values = append(values, item)
	}
	return values
}

func requestWithState(req *http.Request, state Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), requestContextKey{}, state))
}

// requestState reads the parsed Inertia state stored on the request context by
// Middleware, falling back to parsing the request headers directly.
func requestState(ctx context.Context) (Request, error) {
	request := anduin.Request(ctx)
	if request == nil {
		return Request{}, protocolError("parse request", "missing HTTP request")
	}

	if state, ok := request.Context().Value(requestContextKey{}).(Request); ok {
		return state, nil
	}

	return ParseHTTPRequest(request)
}
