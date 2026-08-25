// Package inertia implements the Inertia v3 protocol for Echo and templ.
package inertia

// Page is the Inertia v3 page object sent to the browser and SSR runtime.
type Page struct {
	Component        string                    `json:"component"`
	Props            map[string]any            `json:"props"`
	URL              string                    `json:"url"`
	Version          string                    `json:"version"`
	EncryptHistory   bool                      `json:"encryptHistory,omitempty"`
	ClearHistory     bool                      `json:"clearHistory,omitempty"`
	PreserveFragment bool                      `json:"preserveFragment,omitempty"`
	MergeProps       []string                  `json:"mergeProps,omitempty"`
	PrependProps     []string                  `json:"prependProps,omitempty"`
	DeepMergeProps   []string                  `json:"deepMergeProps,omitempty"`
	MatchPropsOn     []string                  `json:"matchPropsOn,omitempty"`
	ScrollProps      map[string]ScrollMetadata `json:"scrollProps,omitempty"`
	DeferredProps    map[string][]string       `json:"deferredProps,omitempty"`
	RescuedProps     []string                  `json:"rescuedProps,omitempty"`
	SharedProps      []string                  `json:"sharedProps,omitempty"`
	OnceProps        map[string]OnceMetadata   `json:"onceProps,omitempty"`
	Flash            any                       `json:"flash,omitempty"`
}

// OnceMetadata tells the client how to retain a once prop.
type OnceMetadata struct {
	Prop      string `json:"prop"`
	ExpiresAt *int64 `json:"expiresAt"`
}

// ScrollMetadata describes infinite-scroll pagination for a prop.
type ScrollMetadata struct {
	PageName     string `json:"pageName"`
	PreviousPage any    `json:"previousPage"`
	NextPage     any    `json:"nextPage"`
	CurrentPage  any    `json:"currentPage"`
	Reset        bool   `json:"reset"`
}

func (metadata ScrollMetadata) GetPageName() string  { return metadata.PageName }
func (metadata ScrollMetadata) GetPreviousPage() any { return metadata.PreviousPage }
func (metadata ScrollMetadata) GetNextPage() any     { return metadata.NextPage }
func (metadata ScrollMetadata) GetCurrentPage() any  { return metadata.CurrentPage }
