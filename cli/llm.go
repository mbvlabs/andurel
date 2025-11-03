package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLLMCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "llm",
		Short: "Output LLM-optimized project documentation",
		Long:  "Outputs a concise reference guide optimized for LLMs working with andurel-scaffolded projects.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(llmDocumentation)
		},
	}
}

const llmDocumentation = `# Andurel Project - LLM Quick Reference

## File Locations (Quick Lookup)

` + "```" + `
models/<resource>_model.go              - Model structs (edit freely)
models/internal/db/<resource>.sql.go    - SQLC generated (DO NOT EDIT)
models/internal/db/<resource>_constructors.go - Generated (DO NOT EDIT)
controllers/<resource>_controller.go    - Controllers (edit freely, add logic)
router/routes/<resource>_routes.go      - Routes (edit freely)
views/<table>_resource.templ            - Views (edit freely)
database/queries/<table>.sql            - SQL queries (DO NOT EDIT - regenerate with --refresh)
database/migrations/*.sql               - Migrations (create with 'andurel m new')
config/                                 - App config (edit freely)
router/routes/routes.go                 - Route registration (MUST update when adding resources)
database/sqlc.yaml                      - SQLC config (rarely edit)
` + "```" + `

## Common Tasks (Optimized Patterns)

### Add new field to existing model
` + "```bash" + `
# 1. Create migration
andurel m new add_<field>_to_<table>
# 2. Edit database/migrations/YYYYMMDDHHMMSS_add_<field>_to_<table>.sql
# 3. Apply migration
andurel m up
# 4. Refresh model (CRITICAL - updates structs, queries, constructors)
andurel g model <Resource> --refresh
` + "```" + `

### Create new resource
` + "```bash" + `
# 1. Create migration with table schema
andurel m new create_<table>_table
# 2. Edit migration file
# 3. Apply migration
andurel m up
# 4. Generate resource
andurel g resource <Resource>
# 5. Register routes in router/routes/routes.go
` + "```" + `

### Modify controller logic
- File: ` + "`controllers/<resource>_controller.go`" + `
- Safe to edit all functions
- Do NOT delete functions - comment out if not needed

### Add custom query
- File: ` + "`database/queries/<table>.sql`" + `
- Will be regenerated on ` + "`--refresh`" + `, so add custom queries elsewhere OR
- Use raw SQL in controllers if one-off

### Modify views
- File: ` + "`views/<table>_resource.templ`" + `
- After editing, run: ` + "`templ generate`" + ` or restart dev server

## Naming Rules (Auto-Applied)

Input: ` + "`User`" + ` →
- Table: ` + "`users`" + `
- Model: ` + "`user_model.go`" + `
- Controller: ` + "`user_controller.go`" + `
- Routes: ` + "`user_routes.go`" + `
- Views: ` + "`users_resource.templ`" + `
- Queries: ` + "`users.sql`" + `

Input: ` + "`BlogPost`" + ` →
- Table: ` + "`blog_posts`" + `
- Model: ` + "`blog_post_model.go`" + `
- Controller: ` + "`blog_post_controller.go`" + `
- Routes: ` + "`blog_post_routes.go`" + `
- Views: ` + "`blog_posts_resource.templ`" + `
- Queries: ` + "`blog_posts.sql`" + `

**Rule**: Always use PascalCase resource name in commands. Andurel handles conversions.

## Files You Should NEVER Edit

- ` + "`models/internal/db/*.sql.go`" + ` - SQLC generated
- ` + "`models/internal/db/*_constructors.go`" + ` - Generated
- ` + "`views/*_templ.go`" + ` - Generated from .templ files

**Instead**: Use ` + "`--refresh`" + ` or regeneration commands.

## Generated Controller Actions

Every resource controller has:
- ` + "`Index(c echo.Context)`" + ` - GET /resources
- ` + "`Show(c echo.Context)`" + ` - GET /resources/:id
- ` + "`New(c echo.Context)`" + ` - GET /resources/new
- ` + "`Create(c echo.Context)`" + ` - POST /resources
- ` + "`Edit(c echo.Context)`" + ` - GET /resources/:id/edit
- ` + "`Update(c echo.Context)`" + ` - PUT /resources/:id
- ` + "`Destroy(c echo.Context)`" + ` - DELETE /resources/:id

## Commands Reference

` + "```bash" + `
# Migrations (goose)
andurel m new <name>        # Create migration
andurel m up                # Apply all pending
andurel m down              # Rollback last
andurel m reset             # Rollback all + reapply

# Generation
andurel g model <Resource>           # Model only (requires migration exists)
andurel g model <Resource> --refresh # Regenerate after schema change
andurel g controller <Resource> --with-views  # Controller + views
andurel g view <Resource> --with-controller   # Views + controller
andurel g resource <Resource>                 # All of above

# SQLC
andurel s compile           # Check SQL validity
andurel s generate          # Generate Go from SQL (auto-run by g model)

# App
andurel app tailwind        # Setup Tailwind CSS
andurel app console         # Interactive console
` + "```" + `

## Quick Debugging

**"Model field missing after schema change"**
→ Run ` + "`andurel g model <Resource> --refresh`" + `

**"Routes not working"**
→ Check ` + "`router/routes/routes.go`" + ` for route registration

**"SQLC errors"**
→ Run ` + "`andurel s compile`" + ` to validate SQL

**"Views not updating"**
→ Run ` + "`go tool templ generate`" + ` or check ` + "`views/*_templ.go`" + ` exists

## Search Patterns (When You Need to Find Files)

- Controllers: ` + "`controllers/*_controller.go`" + `
- Models: ` + "`models/*_model.go`" + `
- Views: ` + "`views/*_resource.templ`" + `
- Routes: ` + "`router/routes/*_routes.go`" + `
- Migrations: ` + "`database/migrations/*.sql`" + `
- SQLC queries: ` + "`database/queries/*.sql`" + `

## Database-Specific Notes

Check ` + "`database/sqlc.yaml`" + ` for current database engine.`
