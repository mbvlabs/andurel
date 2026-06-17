package blueprint_test

import (
	"testing"

	"github.com/mbvlabs/andurel/layout/blueprint"
)

func TestNewBuilder(t *testing.T) {
	bp := blueprint.New()
	builder := blueprint.NewBuilder(bp)

	if builder == nil {
		t.Fatal("expected NewBuilder to return non-nil builder")
	}

	if builder.Blueprint() != bp {
		t.Error("expected builder to wrap provided blueprint")
	}
}

func TestNewBuilder_NilBlueprint(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	if builder == nil {
		t.Fatal("expected NewBuilder to return non-nil builder even with nil input")
	}

	if builder.Blueprint() == nil {
		t.Error("expected builder to create new blueprint when given nil")
	}
}

func TestBuilder_AddImport(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.AddControllerImport("fmt").
		AddControllerImport("strings").
		AddControllerImport("fmt")
		// duplicate

	imports := builder.Blueprint().Controllers.Imports.Items()

	if len(imports) != 2 {
		t.Fatalf("expected 2 unique imports, got %d", len(imports))
	}
}

func TestBuilder_AddRoute(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	route1 := blueprint.Route{Name: "home", Path: "/", Controller: "Pages.Home"}
	route2 := blueprint.Route{Name: "about", Path: "/about", Controller: "Pages.About"}

	builder.AddRoute(route1).AddRoute(route2).AddRoute(route1) // duplicate

	routes := builder.Blueprint().Routes.SortedRoutes()

	if len(routes) != 2 {
		t.Fatalf("expected 2 unique routes, got %d", len(routes))
	}

	if routes[0].Name != "home" {
		t.Errorf("expected first route to be 'home', got '%s'", routes[0].Name)
	}
}

func TestBuilder_AddRouteCollection(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.
		AddRouteCollection("RouteA", "RouteB", "RouteA").
		AddRouteCollection("RouteC")

	collections := builder.Blueprint().Routes.SortedRouteCollections()
	if len(collections) != 2 {
		t.Fatalf("expected 2 route collections, got %d", len(collections))
	}

	if got := collections[0].Routes; len(got) != 2 || got[0] != "RouteA" || got[1] != "RouteB" {
		t.Fatalf("unexpected routes in first collection: %#v", got)
	}

	if got := collections[1].Routes; len(got) != 1 || got[0] != "RouteC" {
		t.Fatalf("unexpected routes in second collection: %#v", got)
	}
}

func TestBuilder_AddRouteImport(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.AddRouteImport("middleware").AddRouteImport("middleware") // duplicate

	imports := builder.Blueprint().Routes.Imports.Items()

	if len(imports) != 1 {
		t.Fatalf("expected 1 unique import, got %d", len(imports))
	}
}

func TestBuilder_AddModel(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	model1 := blueprint.Model{Name: "User"}
	model2 := blueprint.Model{Name: "Post"}

	builder.AddModel(model1).AddModel(model2).AddModel(model1) // duplicate

	models := builder.Blueprint().Models.SortedModels()

	if len(models) != 2 {
		t.Fatalf("expected 2 unique models, got %d", len(models))
	}

	if models[0].Name != "User" {
		t.Errorf("expected first model to be 'User', got '%s'", models[0].Name)
	}
}

func TestBuilder_AddModelImport(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.AddModelImport("time").AddModelImport("time") // duplicate

	imports := builder.Blueprint().Models.Imports.Items()

	if len(imports) != 1 {
		t.Fatalf("expected 1 unique import, got %d", len(imports))
	}
}

func TestBuilder_AddConfigField(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.
		AddConfigField("Port", "int").
		AddConfigField("Host", "string").
		AddConfigField("Port", "int") // duplicate

	fields := builder.Blueprint().Config.SortedFields()

	if len(fields) != 2 {
		t.Fatalf("expected 2 unique config fields, got %d", len(fields))
	}

	if fields[0].Name != "Port" {
		t.Errorf("expected first config field to be 'Port', got '%s'", fields[0].Name)
	}
}

func TestBuilder_AddEnvVar(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.
		AddEnvVar("PORT", "Port", "8080").
		AddEnvVar("HOST", "Host", "localhost").
		AddEnvVar("PORT", "Port", "8080") // duplicate

	envVars := builder.Blueprint().Config.SortedEnvVars()

	if len(envVars) != 2 {
		t.Fatalf("expected 2 unique env vars, got %d", len(envVars))
	}

	if envVars[0].Key != "PORT" {
		t.Errorf("expected first env var to be 'PORT', got '%s'", envVars[0].Key)
	}
}

func TestBuilder_AddMigration(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	mig1 := blueprint.Migration{Name: "001_init", Timestamp: "20250101000000"}
	mig2 := blueprint.Migration{Name: "002_users", Timestamp: "20250102000000"}

	builder.AddMigration(mig1).AddMigration(mig2).AddMigration(mig1) // duplicate

	migrations := builder.Blueprint().Migrations.SortedMigrations()

	if len(migrations) != 2 {
		t.Fatalf("expected 2 unique migrations, got %d", len(migrations))
	}

	if migrations[0].Name != "001_init" {
		t.Errorf("expected first migration to be '001_init', got '%s'", migrations[0].Name)
	}
}

func TestBuilder_Merge(t *testing.T) {
	builder1 := blueprint.NewBuilder(nil)
	builder1.
		AddControllerImport("fmt").
		AddServiceProvide("email.NewMailpit").
		AddWorkerDependency("db", "database.DB")

	builder2 := blueprint.NewBuilder(nil)
	builder2.
		AddControllerImport("strings").       // new
		AddControllerImport("fmt").           // duplicate
		AddServiceProvide("queue.NewWorker"). // new
		AddWorkerDependency("cache", "cache.Cache") // new

	err := builder1.Merge(builder2.Blueprint())
	if err != nil {
		t.Fatalf("unexpected error during merge: %v", err)
	}

	// Check imports
	imports := builder1.Blueprint().Controllers.Imports.Items()
	if len(imports) != 2 {
		t.Errorf("expected 2 unique imports after merge, got %d", len(imports))
	}

	// Check service provides
	provides := builder1.Blueprint().Main.ServiceProvides
	if len(provides) != 2 {
		t.Errorf("expected 2 unique service provides after merge, got %d", len(provides))
	}

	// Check worker dependencies
	deps := builder1.Blueprint().Main.WorkerDependencies
	if len(deps) != 2 {
		t.Errorf("expected 2 unique worker dependencies after merge, got %d", len(deps))
	}
}

func TestBuilder_Merge_NilBlueprint(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	err := builder.Merge(nil)
	if err == nil {
		t.Error("expected error when merging nil blueprint")
	}
}

func TestBuilder_Chaining(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	// Test that all methods return builder for chaining
	result := builder.
		AddControllerImport("fmt").
		AddWorkerDependency("db", "database.DB").
		AddServiceProvide("email.NewMailpit").
		AddRoute(blueprint.Route{Name: "home", Path: "/"}).
		AddRouteCollection("HomePage").
		AddRouteImport("middleware").
		AddModel(blueprint.Model{Name: "User"}).
		AddModelImport("time").
		AddConfigField("Port", "int").
		AddEnvVar("PORT", "Port", "8080").
		AddMigration(blueprint.Migration{Name: "001_init"})

	if result != builder {
		t.Error("expected all builder methods to return the builder for chaining")
	}
}

func TestBuilder_EmptyValues(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	// These should be no-ops
	builder.
		AddWorkerDependency("", "Type").
		AddWorkerDependency("name", "").
		AddServiceProvide("").
		AddRoute(blueprint.Route{Name: "", Path: "/"}).    // missing name
		AddRoute(blueprint.Route{Name: "name", Path: ""}). // missing path
		AddRouteCollection("", "  ").
		AddRouteImport("").
		AddModel(blueprint.Model{Name: ""}). // missing name
		AddModelImport("").
		AddConfigField("", "Type").
		AddConfigField("name", "").
		AddEnvVar("", "field", "default").
		AddEnvVar("key", "", "default").
		AddMigration(blueprint.Migration{Name: ""}) // missing name

	bp := builder.Blueprint()

	if bp.Controllers.Imports.Len() != 0 {
		t.Error("expected no imports for empty values")
	}

	if len(bp.Main.WorkerDependencies) != 0 {
		t.Error("expected no worker dependencies for empty values")
	}

	if len(bp.Main.ServiceProvides) != 0 {
		t.Error("expected no service provides for empty values")
	}

	if len(bp.Routes.Routes) != 0 {
		t.Error("expected no routes for empty values")
	}
	if len(bp.Routes.RouteCollections) != 0 {
		t.Error("expected no route collections for empty values")
	}

	if len(bp.Models.Models) != 0 {
		t.Error("expected no models for empty values")
	}

	if len(bp.Config.Fields) != 0 {
		t.Error("expected no config fields for empty values")
	}

	if len(bp.Config.EnvVars) != 0 {
		t.Error("expected no env vars for empty values")
	}

	if len(bp.Migrations.Migrations) != 0 {
		t.Error("expected no migrations for empty values")
	}
}

func TestBuilder_MainSection(t *testing.T) {
	b := blueprint.NewBuilder(nil)

	// Test AddMainImport
	b.AddMainImport("myapp/email")
	b.AddMainImport("myapp/queue")
	b.AddMainImport("myapp/email") // duplicate

	imports := b.Blueprint().Main.Imports.Items()
	if len(imports) != 2 {
		t.Errorf("expected 2 imports, got %d", len(imports))
	}

	// Test AddServiceProvide
	b.AddServiceProvide("email.NewMailpit")
	b.AddServiceProvide("queue.NewWorker")
	b.AddServiceProvide("email.NewMailpit") // duplicate

	provides := b.Blueprint().Main.ServiceProvides
	if len(provides) != 2 {
		t.Errorf("expected 2 service provides, got %d", len(provides))
	}
	if provides[0] != "email.NewMailpit" {
		t.Errorf("expected first provide to be email.NewMailpit, got %s", provides[0])
	}

	// Test AddWorkerDependency
	b.AddWorkerDependency("transactionalSender", "email.TransactionalSender")
	b.AddWorkerDependency("marketingSender", "email.MarketingSender")
	b.AddWorkerDependency("transactionalSender", "email.TransactionalSender") // duplicate

	deps := b.Blueprint().Main.WorkerDependencies
	if len(deps) != 2 {
		t.Errorf("expected 2 worker dependencies, got %d", len(deps))
	}

	// Test AddPreRunHook
	b.AddPreRunHook("migrate", "if err := migrate(db); err != nil { return err }")
	b.AddPreRunHook("seed", "seed(db)")
	b.AddPreRunHook("migrate", "if err := migrate2(db); err != nil { return err }") // duplicate

	hooks := b.Blueprint().Main.SortedPreRunHooks()
	if len(hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(hooks))
	}
}

func TestBuilder_MergeMainSection(t *testing.T) {
	b1 := blueprint.NewBuilder(nil)
	b1.AddMainImport("myapp/email")
	b1.AddServiceProvide("email.NewMailpit")

	b2 := blueprint.NewBuilder(nil)
	b2.AddMainImport("myapp/queue")
	b2.AddServiceProvide("queue.New")

	err := b1.Merge(b2.Blueprint())
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// Check merged imports
	imports := b1.Blueprint().Main.Imports.Items()
	if len(imports) != 2 {
		t.Errorf("expected 2 imports after merge, got %d", len(imports))
	}

	// Check merged service provides
	provides := b1.Blueprint().Main.ServiceProvides
	if len(provides) != 2 {
		t.Errorf("expected 2 service provides after merge, got %d", len(provides))
	}
}
