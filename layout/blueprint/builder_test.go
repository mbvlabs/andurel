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

func TestBuilder_AddControllerDependency(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.
		AddControllerDependency("db", "database.DB").
		AddControllerDependency("cache", "cache.Cache").
		AddControllerDependency("db", "database.DB") // duplicate

	deps := builder.Blueprint().Controllers.SortedDependencies()

	if len(deps) != 2 {
		t.Fatalf("expected 2 unique dependencies, got %d", len(deps))
	}

	if deps[0].Name != "db" {
		t.Errorf("expected first dependency to be 'db', got '%s'", deps[0].Name)
	}

	if deps[1].Name != "cache" {
		t.Errorf("expected second dependency to be 'cache', got '%s'", deps[1].Name)
	}

	// Check ordering
	if deps[0].Order != 0 {
		t.Errorf("expected first dependency order to be 0, got %d", deps[0].Order)
	}

	if deps[1].Order != 1 {
		t.Errorf("expected second dependency order to be 1, got %d", deps[1].Order)
	}
}

func TestBuilder_AddControllerField(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.
		AddControllerField("Pages", "Pages").
		AddControllerField("API", "API").
		AddControllerField("Pages", "Pages") // duplicate

	fields := builder.Blueprint().Controllers.SortedFields()

	if len(fields) != 2 {
		t.Fatalf("expected 2 unique fields, got %d", len(fields))
	}

	if fields[0].Name != "Pages" {
		t.Errorf("expected first field to be 'Pages', got '%s'", fields[0].Name)
	}
}

func TestBuilder_AddConstructor(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.
		AddControllerConstructor("pages", "newPages(db)").
		AddControllerConstructor("api", "newAPI(db)").
		AddControllerConstructor("pages", "newPages(db)") // duplicate

	ctors := builder.Blueprint().Controllers.SortedConstructors()

	if len(ctors) != 2 {
		t.Fatalf("expected 2 unique constructors, got %d", len(ctors))
	}

	if ctors[0].VarName != "pages" {
		t.Errorf("expected first constructor to be 'pages', got '%s'", ctors[0].VarName)
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
		AddRouteCollection("RouteA", " RouteB ", "RouteA", "").
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

func TestBuilder_AddRouteRegistration(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	result := builder.
		AddRouteRegistration("http.MethodGet", "routes.Home", "ctrls.Pages.Home", "auth").
		AddRouteRegistration("", "routes.Empty", "ctrls.Empty").
		AddRouteRegistration("http.MethodPost", "", "ctrls.Empty").
		AddRouteRegistration("http.MethodPost", "routes.Empty", "")

	if result != builder {
		t.Fatal("expected AddRouteRegistration to return builder")
	}

	registrations := builder.Blueprint().Routes.SortedRegistrations()
	if len(registrations) != 1 {
		t.Fatalf("expected 1 route registration, got %d", len(registrations))
	}
	if registrations[0].Method != "http.MethodGet" {
		t.Fatalf("Method = %q", registrations[0].Method)
	}
	if registrations[0].RouteVariable != "routes.Home" {
		t.Fatalf("RouteVariable = %q", registrations[0].RouteVariable)
	}
	if registrations[0].HandlerRef != "ctrls.Pages.Home" {
		t.Fatalf("HandlerRef = %q", registrations[0].HandlerRef)
	}
	if got := registrations[0].Middleware; len(got) != 1 || got[0] != "auth" {
		t.Fatalf("Middleware = %#v", got)
	}
}

func TestBuilder_RouteRegistrationFunction(t *testing.T) {
	builder := blueprint.NewBuilder(nil)

	builder.
		StartRouteRegistrationFunction("", "ctrls").
		StartRouteRegistrationFunction("registerAdminRoutes", "admin").
		StartRouteRegistrationFunction("ignoredNested", "nested").
		AddRouteRegistration("http.MethodGet", "routes.AdminDashboard", "admin.Dashboard").
		EndRouteRegistrationFunction().
		EndRouteRegistrationFunction()

	functions := builder.Blueprint().Routes.SortedRegistrationFunctions()
	if len(functions) != 1 {
		t.Fatalf("expected 1 registration function, got %d", len(functions))
	}
	if functions[0].FunctionName != "registerAdminRoutes" {
		t.Fatalf("FunctionName = %q", functions[0].FunctionName)
	}
	if functions[0].ControllerVarName != "admin" {
		t.Fatalf("ControllerVarName = %q", functions[0].ControllerVarName)
	}
	if len(functions[0].Registrations) != 1 {
		t.Fatalf("expected 1 nested registration, got %d", len(functions[0].Registrations))
	}

	builder.
		StartRouteRegistrationFunction("registerAdminRoutes", "other").
		AddRouteRegistration("http.MethodPost", "routes.AdminCreate", "other.Create").
		EndRouteRegistrationFunction()

	functions = builder.Blueprint().Routes.SortedRegistrationFunctions()
	if len(functions) != 1 {
		t.Fatalf("duplicate function should be ignored, got %d", len(functions))
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
		AddControllerDependency("db", "database.DB").
		AddControllerField("Pages", "Pages").
		AddControllerConstructor("pages", "newPages(db)")

	builder2 := blueprint.NewBuilder(nil)
	builder2.
		AddControllerImport("strings").                  // new
		AddControllerImport("fmt").                      // duplicate
		AddControllerDependency("cache", "cache.Cache"). // new
		AddControllerDependency("db", "database.DB").    // duplicate
		AddControllerField("API", "API").                // new
		AddControllerConstructor("api", "newAPI(db)")    // new

	err := builder1.Merge(builder2.Blueprint())
	if err != nil {
		t.Fatalf("unexpected error during merge: %v", err)
	}

	// Check imports
	imports := builder1.Blueprint().Controllers.Imports.Items()
	if len(imports) != 2 {
		t.Errorf("expected 2 unique imports after merge, got %d", len(imports))
	}

	// Check dependencies
	deps := builder1.Blueprint().Controllers.SortedDependencies()
	if len(deps) != 2 {
		t.Errorf("expected 2 unique dependencies after merge, got %d", len(deps))
	}

	// Check fields
	fields := builder1.Blueprint().Controllers.SortedFields()
	if len(fields) != 2 {
		t.Errorf("expected 2 unique fields after merge, got %d", len(fields))
	}

	// Check constructors
	ctors := builder1.Blueprint().Controllers.SortedConstructors()
	if len(ctors) != 2 {
		t.Errorf("expected 2 unique constructors after merge, got %d", len(ctors))
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
		AddControllerDependency("db", "database.DB").
		AddControllerField("Pages", "Pages").
		AddControllerConstructor("pages", "newPages(db)").
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
		AddControllerDependency("", "Type").
		AddControllerDependency("name", "").
		AddControllerField("", "Type").
		AddControllerField("name", "").
		AddControllerConstructor("", "expr").
		AddControllerConstructor("var", "").
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

	if len(bp.Controllers.Dependencies) != 0 {
		t.Error("expected no dependencies for empty values")
	}

	if len(bp.Controllers.Fields) != 0 {
		t.Error("expected no fields for empty values")
	}

	if len(bp.Controllers.Constructors) != 0 {
		t.Error("expected no constructors for empty values")
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

	// Test AddMainInitialization
	b.AddMainInitialization("emailSender", "email.NewMailpit()", "cfg")
	b.AddMainInitialization("queue", "queue.New()", "db", "cfg")
	b.AddMainInitialization("emailSender", "email.NewSMTP()") // duplicate

	inits := b.Blueprint().Main.SortedInitializations()
	if len(inits) != 2 {
		t.Errorf("expected 2 initializations, got %d", len(inits))
	}
	if inits[0].VarName != "emailSender" {
		t.Errorf("expected first init to be emailSender, got %s", inits[0].VarName)
	}
	if len(inits[1].DependsOn) != 2 {
		t.Errorf("expected queue to have 2 dependencies, got %d", len(inits[1].DependsOn))
	}

	// Test AddBackgroundWorker
	b.AddBackgroundWorker("queue-worker", "worker.Start(ctx, queue)", "queue")
	b.AddBackgroundWorker("scheduler", "scheduler.Start(ctx)")
	b.AddBackgroundWorker("queue-worker", "worker.StartAgain(ctx)") // duplicate

	workers := b.Blueprint().Main.SortedBackgroundWorkers()
	if len(workers) != 2 {
		t.Errorf("expected 2 workers, got %d", len(workers))
	}

	// Test AddPreRunHook
	b.AddPreRunHook("migrate", "if err := migrate(db); err != nil { return err }")
	b.AddPreRunHook("seed", "seed(db)")
	b.AddPreRunHook("migrate", "if err := migrate2(db); err != nil { return err }") // duplicate

	hooks := b.Blueprint().Main.SortedPreRunHooks()
	if len(hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(hooks))
	}

	b.AddServiceProvide("email.NewSender").
		AddServiceProvide("queue.NewWorker").
		AddServiceProvide("email.NewSender").
		AddServiceProvide("")
	if got := b.Blueprint().Main.ServiceProvides; len(got) != 2 {
		t.Fatalf("expected 2 service provides, got %#v", got)
	}

	b.AddWorkerDependency("queue", "queue.Queue").
		AddWorkerDependency("queue", "queue.PriorityQueue").
		AddWorkerDependency("", "missing.Name").
		AddWorkerDependency("cache", "")
	if got := b.Blueprint().Main.WorkerDependencies; len(got) != 1 {
		t.Fatalf("expected 1 worker dependency, got %#v", got)
	} else if got[0].Name != "queue" || got[0].Type != "queue.PriorityQueue" {
		t.Fatalf("unexpected worker dependency: %#v", got[0])
	}
}

func TestBuilder_CookiesSection(t *testing.T) {
	b := blueprint.NewBuilder(nil)

	b.AddCookiesImport("net/http").AddCookiesImport("net/http").AddCookiesImport("")
	if got := b.Blueprint().Cookies.Imports.Items(); len(got) != 1 || got[0] != "net/http" {
		t.Fatalf("cookies imports = %#v", got)
	}

	b.AddCookiesConstant("SessionName", `"session"`).
		AddCookiesConstant("CSRFName", `"csrf"`).
		AddCookiesConstant("SessionName", `"other"`).
		AddCookiesConstant("", `"empty"`).
		AddCookiesConstant("EmptyValue", "")
	if got := b.Blueprint().Cookies.SortedConstants(); len(got) != 2 {
		t.Fatalf("expected 2 constants, got %#v", got)
	} else if got[0].Name != "SessionName" || got[1].Name != "CSRFName" {
		t.Fatalf("unexpected constants: %#v", got)
	}

	b.AddCookiesAppField("Store", "*sessions.CookieStore").
		AddCookiesAppField("Signer", "Signer").
		AddCookiesAppField("Store", "Other").
		AddCookiesAppField("", "Missing").
		AddCookiesAppField("Missing", "")
	if got := b.Blueprint().Cookies.SortedAppFields(); len(got) != 2 {
		t.Fatalf("expected 2 app fields, got %#v", got)
	} else if got[0].Name != "Store" || got[1].Name != "Signer" {
		t.Fatalf("unexpected app fields: %#v", got)
	}

	b.AddCookiesFunction("CreateSession", "return nil").
		AddCookiesFunction("GetSession", "return nil").
		AddCookiesFunction("CreateSession", "panic(\"duplicate\")").
		AddCookiesFunction("", "return nil").
		AddCookiesFunction("Empty", "").
		SetCookiesCreateSessionCode("session.Values[\"id\"] = id").
		SetCookiesGetSessionCode("return session.Values[\"id\"]")
	if got := b.Blueprint().Cookies.SortedFunctions(); len(got) != 2 {
		t.Fatalf("expected 2 functions, got %#v", got)
	} else if got[0].Name != "CreateSession" || got[1].Name != "GetSession" {
		t.Fatalf("unexpected functions: %#v", got)
	}
	if got := b.Blueprint().Cookies.CreateSessionCode; got != "session.Values[\"id\"] = id" {
		t.Fatalf("CreateSessionCode = %q", got)
	}
	if got := b.Blueprint().Cookies.GetSessionCode; got != "return session.Values[\"id\"]" {
		t.Fatalf("GetSessionCode = %q", got)
	}
}

func TestBuilder_MergeMainSection(t *testing.T) {
	b1 := blueprint.NewBuilder(nil)
	b1.AddMainImport("myapp/email")
	b1.AddMainInitialization("emailSender", "email.New()")
	b1.AddServiceProvide("email.NewSender")
	b1.AddWorkerDependency("queue", "queue.Queue")

	b2 := blueprint.NewBuilder(nil)
	b2.AddMainImport("myapp/queue")
	b2.AddMainInitialization("queue", "queue.New()")
	b2.AddBackgroundWorker("worker", "worker.Start()")
	b2.AddServiceProvide("queue.NewWorker")
	b2.AddServiceProvide("email.NewSender")
	b2.AddWorkerDependency("queue", "queue.PriorityQueue")
	b2.AddWorkerDependency("cache", "cache.Cache")

	err := b1.Merge(b2.Blueprint())
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// Check merged imports
	imports := b1.Blueprint().Main.Imports.Items()
	if len(imports) != 2 {
		t.Errorf("expected 2 imports after merge, got %d", len(imports))
	}

	// Check merged initializations
	inits := b1.Blueprint().Main.SortedInitializations()
	if len(inits) != 2 {
		t.Errorf("expected 2 initializations after merge, got %d", len(inits))
	}

	// Check merged workers
	workers := b1.Blueprint().Main.SortedBackgroundWorkers()
	if len(workers) != 1 {
		t.Errorf("expected 1 worker after merge, got %d", len(workers))
	}

	if got := b1.Blueprint().Main.ServiceProvides; len(got) != 2 {
		t.Fatalf("expected 2 service provides after merge, got %#v", got)
	}

	deps := b1.Blueprint().Main.WorkerDependencies
	if len(deps) != 2 {
		t.Fatalf("expected 2 worker dependencies after merge, got %#v", deps)
	}
	if deps[0].Name != "queue" || deps[0].Type != "queue.PriorityQueue" {
		t.Fatalf("expected queue dependency to be updated, got %#v", deps[0])
	}
}

func TestBuilder_MergeRouteRegistrationsAndCookies(t *testing.T) {
	b1 := blueprint.NewBuilder(nil)
	b1.AddRouteGroup("public")
	b1.AddRouteRegistration("http.MethodGet", "routes.Home", "ctrls.Pages.Home")
	b1.StartRouteRegistrationFunction("registerAdminRoutes", "admin").
		AddRouteRegistration("http.MethodGet", "routes.AdminDashboard", "admin.Dashboard").
		EndRouteRegistrationFunction()
	b1.AddCookiesConstant("SessionName", `"session"`)
	b1.SetCookiesCreateSessionCode("old")

	b2 := blueprint.NewBuilder(nil)
	b2.AddRouteGroup("admin")
	b2.AddRouteRegistration("http.MethodPost", "routes.Login", "ctrls.Auth.Login", "guest")
	b2.StartRouteRegistrationFunction("registerAPIRoutes", "api").
		AddRouteRegistration("http.MethodGet", "routes.APIHealth", "api.Health").
		EndRouteRegistrationFunction()
	b2.AddCookiesImport("net/http")
	b2.AddCookiesConstant("CSRFName", `"csrf"`)
	b2.AddCookiesAppField("Store", "*sessions.CookieStore")
	b2.AddCookiesFunction("CreateSession", "return nil")
	b2.SetCookiesCreateSessionCode("new")
	b2.SetCookiesGetSessionCode("get")

	if err := b1.Merge(b2.Blueprint()); err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	if got := b1.Blueprint().Routes.RouteGroups.Items(); len(got) != 2 {
		t.Fatalf("expected 2 route groups, got %#v", got)
	}
	if got := b1.Blueprint().Routes.SortedRegistrations(); len(got) != 2 {
		t.Fatalf("expected 2 route registrations, got %#v", got)
	}
	if got := b1.Blueprint().Routes.SortedRegistrationFunctions(); len(got) != 2 {
		t.Fatalf("expected 2 registration functions, got %#v", got)
	}
	if got := b1.Blueprint().Cookies.Imports.Items(); len(got) != 1 || got[0] != "net/http" {
		t.Fatalf("cookies imports = %#v", got)
	}
	if got := b1.Blueprint().Cookies.SortedConstants(); len(got) != 2 {
		t.Fatalf("expected 2 cookie constants, got %#v", got)
	}
	if got := b1.Blueprint().Cookies.SortedAppFields(); len(got) != 1 {
		t.Fatalf("expected 1 cookie app field, got %#v", got)
	}
	if got := b1.Blueprint().Cookies.SortedFunctions(); len(got) != 1 {
		t.Fatalf("expected 1 cookie function, got %#v", got)
	}
	if got := b1.Blueprint().Cookies.CreateSessionCode; got != "new" {
		t.Fatalf("CreateSessionCode = %q", got)
	}
	if got := b1.Blueprint().Cookies.GetSessionCode; got != "get" {
		t.Fatalf("GetSessionCode = %q", got)
	}
}
