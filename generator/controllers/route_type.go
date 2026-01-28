package controllers

import "strings"

// RouteType represents the type of route constructor to use
type RouteType int

const (
	SimpleRoute RouteType = iota
	RouteWithID
	RouteWithSlug
	RouteWithToken
	RouteWithFile
	RouteWithMultipleIDs
)

// DetectRouteType analyzes a path string and returns the appropriate RouteType.
// It inspects path segments for :param patterns and maps them to routing constructors.
func DetectRouteType(path string) RouteType {
	segments := strings.Split(path, "/")

	var idCount int
	var hasSlug, hasToken, hasFile bool

	for _, seg := range segments {
		if !strings.HasPrefix(seg, ":") {
			continue
		}

		param := seg[1:]
		switch param {
		case "id":
			idCount++
		case "slug":
			hasSlug = true
		case "token":
			hasToken = true
		case "file":
			hasFile = true
		default:
			// Treat unknown params as IDs
			idCount++
		}
	}

	if idCount > 1 {
		return RouteWithMultipleIDs
	}
	if hasSlug {
		return RouteWithSlug
	}
	if hasToken {
		return RouteWithToken
	}
	if hasFile {
		return RouteWithFile
	}
	if idCount == 1 {
		return RouteWithID
	}
	return SimpleRoute
}

// ConstructorName returns the routing package constructor function name for this RouteType.
func (rt RouteType) ConstructorName() string {
	switch rt {
	case RouteWithID:
		return "NewRouteWithID"
	case RouteWithSlug:
		return "NewRouteWithSlug"
	case RouteWithToken:
		return "NewRouteWithToken"
	case RouteWithFile:
		return "NewRouteWithFile"
	case RouteWithMultipleIDs:
		return "NewRouteWithMultipleIDs"
	default:
		return "NewSimpleRoute"
	}
}
