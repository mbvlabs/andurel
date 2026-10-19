package handlers

import (
	"context"

	"example.com/app/routes"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/inertia"
)

// Projects is a hand-written resource handler the sync goldens scan for
// routes and Inertia payload types.
type Projects struct {
	renderer *inertia.Renderer
}

func NewProjects(renderer *inertia.Renderer) Projects {
	return Projects{renderer: renderer}
}

func (p Projects) Register(rt *anduin.Router) {
	rt.Get(routes.ProjectIndex.Path(), p.Index)
	rt.Get(routes.ProjectShow.Path(), p.Show)
	rt.Post(routes.ProjectCreate.Path(), p.Create)
	rt.Put(routes.ProjectUpdate.Path(), p.Update)
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

func (p Projects) Index(ctx context.Context) error {
	return p.renderer.Page(ctx, "Projects/Index", inertia.FromStruct(ProjectIndexProps{})).Render()
}

func (p Projects) Show(ctx context.Context) error {
	props := ProjectItemProps{}
	return p.renderer.Page(ctx, "Projects/Show", inertia.FromStruct(props)).Render()
}

func (p Projects) Create(ctx context.Context) error {
	var payload CreateProjectFormPayload
	if err := anduin.DecodeJSON(ctx, &payload); err != nil {
		return anduin.BadRequest(err)
	}

	return anduin.Redirect(ctx, routes.ProjectIndex.URL(), 303)
}

func (p Projects) Update(ctx context.Context) error {
	var payload UpdateProjectFormPayload
	if err := anduin.DecodeForm(ctx, &payload); err != nil {
		return anduin.BadRequest(err)
	}

	return anduin.Redirect(ctx, routes.ProjectIndex.URL(), 303)
}
