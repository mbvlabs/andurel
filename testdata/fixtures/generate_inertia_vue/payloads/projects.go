// Package payloads declares the Inertia page props and request bodies the
// frontend consumes. andurel sync payloads emits every exported struct in this
// package to resources/js/types/payloads.ts.
package payloads

type WelcomeProps struct {
	AppName string `json:"appName"`
	DocsURL string `json:"docsURL"`
}

type ProjectData struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Status    string  `json:"status"`
	CreatedAt *string `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt"`
}

type ProjectIndexProps struct {
	Items []ProjectData `json:"items"`
}

type ProjectItemProps struct {
	Item ProjectData `json:"item"`
}

type CreateProjectFormPayload struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

type UpdateProjectFormPayload struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}
