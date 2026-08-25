// Package inertia implements the Inertia v3 protocol for Echo and templ.
package inertia

const (
	HeaderInertia                   = "X-Inertia"
	HeaderVersion                   = "X-Inertia-Version"
	HeaderPartialComponent          = "X-Inertia-Partial-Component"
	HeaderPartialData               = "X-Inertia-Partial-Data"
	HeaderPartialExcept             = "X-Inertia-Partial-Except"
	HeaderReset                     = "X-Inertia-Reset"
	HeaderErrorBag                  = "X-Inertia-Error-Bag"
	HeaderInfiniteScrollMergeIntent = "X-Inertia-Infinite-Scroll-Merge-Intent"
	HeaderExceptOnceProps           = "X-Inertia-Except-Once-Props"
	HeaderLocation                  = "X-Inertia-Location"
	HeaderRedirect                  = "X-Inertia-Redirect"
	HeaderPurpose                   = "Purpose"

	PurposePrefetch = "prefetch"

	MergeIntentAppend  = "append"
	MergeIntentPrepend = "prepend"
)
