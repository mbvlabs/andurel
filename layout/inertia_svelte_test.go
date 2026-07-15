package layout

import "testing"

func TestScaffoldSvelteInertiaAssets(t *testing.T) {
	projectDir := t.TempDir()

	if err := Scaffold(projectDir, "testapp", "postgresql", "test", nil, "svelte", "npm"); err != nil {
		t.Fatalf("scaffold svelte inertia project: %v", err)
	}

	for path, want := range map[string]string{
		"resources/js/app.ts":                                 "mount(App, { target: el, props })",
		"resources/js/Components/FlashToasts.svelte":          "$effect(() =>",
		"resources/js/Layouts/Layout.svelte":                  "{@render children()}",
		"resources/js/Pages/Auth/ConfirmEmail.svelte":         "$form.post(routes.confirmationCreate())",
		"resources/js/Pages/Auth/Login.svelte":                "$form.post(routes.sessionCreate())",
		"resources/js/Pages/Auth/Registration.svelte":         "$form.post(routes.registrationCreate())",
		"resources/js/Pages/Auth/ResetPassword.svelte":        "$form.put(routes.passwordUpdate())",
		"resources/js/Pages/Auth/ResetPasswordRequest.svelte": "$form.post(routes.passwordCreate())",
		"resources/js/Pages/Errors/BadRequest.svelte":         "Bad request",
		"resources/js/Pages/Errors/InternalError.svelte":      "Something went wrong.",
		"resources/js/Pages/Errors/NotFound.svelte":           "Not found",
		"resources/js/routes.ts":                              "sessionCreate: () => '/users/sign-in'",
		"internal/inertia/render.go":                          "package inertia",
		"internal/inertia/vite.go":                            `manifest["resources/js/app.ts"]`,
		"views/root.go.html":                                  `id="app"`,
		"package.json":                                        `"@inertiajs/svelte": "^2.0.0"`,
		"svelte.config.js":                                    "vitePreprocess()",
		"tsconfig.json":                                       `"resources/js/**/*.svelte"`,
		"vite.config.ts":                                      "input: 'resources/js/app.ts'",
	} {
		assertFileContains(t, projectDir, path, want)
	}

	assertFileContains(t, projectDir, "package.json", `"type": "module"`)
	assertFileContains(t, projectDir, "package.json", `"svelte": "^5.0.0"`)
	assertFileContains(t, projectDir, "package.json", `"@sveltejs/vite-plugin-svelte": "^5.0.0"`)
	assertFileContains(t, projectDir, "tsconfig.json", `"types": ["vite/client"]`)
	assertFileNotContains(t, projectDir, "package.json", "@inertiajs/vue3")
	assertFileNotContains(t, projectDir, "package.json", "@inertiajs/react")
	assertFileMissing(t, projectDir, "resources/js/app.tsx")
	assertFileMissing(t, projectDir, "resources/js/Pages/Auth/Login.vue")
	assertFileMissing(t, projectDir, "resources/js/Pages/Auth/Login.tsx")

	lock, err := ReadLockFile(projectDir)
	if err != nil {
		t.Fatalf("read svelte lock: %v", err)
	}
	if lock.ScaffoldConfig == nil || lock.ScaffoldConfig.Inertia != "svelte" || lock.ScaffoldConfig.JavaScriptRuntime != "npm" {
		t.Fatalf("unexpected scaffold config: %#v", lock.ScaffoldConfig)
	}
}
