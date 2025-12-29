// Package hypermedia provides hypermedia SSE protocol support and helpers.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package hypermedia

import (
	"fmt"
	"strings"
)

type DataActionOption string

var ActionTypeForm DataActionOption = "contentType: 'form'"

func ActionHeaders(headers map[string]string) DataActionOption {
	var headerPairs []string
	for key, value := range headers {
		headerPairs = append(headerPairs, fmt.Sprintf("'%s': '%s'", key, value))
	}
	return DataActionOption(fmt.Sprintf("headers: { %s }", strings.Join(headerPairs, ", ")))
}

func ActionSignalsFilter(signals map[string]string) DataActionOption {
	var headerPairs []string
	for key, value := range signals {
		headerPairs = append(headerPairs, fmt.Sprintf("%s: %s", key, value))
	}
	return DataActionOption(fmt.Sprintf("filterSignals: { %s }", strings.Join(headerPairs, ", ")))
}

func DataAction(method, route string, options ...DataActionOption) string {
	if len(options) > 0 {
		var opts []string
		for _, opt := range options {
			opts = append(opts, string(opt))
		}
		options := strings.Join(opts, ", ")

		return fmt.Sprintf(
			"@%s('%s', %s)",
			strings.ToLower(method),
			route,
			fmt.Sprintf("{ %s }", options),
		)
	}

	return fmt.Sprintf("@%s('%s')", strings.ToLower(method), route)
}
