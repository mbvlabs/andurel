package cli

import "testing"

func TestInertiaPackageManagerCommands(t *testing.T) {
	tests := []struct {
		name        string
		runtime     string
		wantInstall string
		wantBuild   string
	}{
		{
			name:        "default runtime uses npm",
			runtime:     "",
			wantInstall: "npm ci",
			wantBuild:   "npm run build",
		},
		{
			name:        "npm",
			runtime:     "npm",
			wantInstall: "npm ci",
			wantBuild:   "npm run build",
		},
		{
			name:        "pnpm",
			runtime:     "pnpm",
			wantInstall: "pnpm install --frozen-lockfile",
			wantBuild:   "pnpm run build",
		},
		{
			name:        "bun",
			runtime:     "bun",
			wantInstall: "bun install --frozen-lockfile",
			wantBuild:   "bun run build",
		},
		{
			name:        "yarn",
			runtime:     "yarn",
			wantInstall: "yarn install --frozen-lockfile",
			wantBuild:   "yarn build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			install, build, err := inertiaPackageManagerCommands(tt.runtime)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if install.String() != tt.wantInstall {
				t.Fatalf("install command = %q, want %q", install.String(), tt.wantInstall)
			}
			if build.String() != tt.wantBuild {
				t.Fatalf("build command = %q, want %q", build.String(), tt.wantBuild)
			}
		})
	}
}

func TestInertiaPackageManagerCommandsRejectsUnsupportedRuntime(t *testing.T) {
	if _, _, err := inertiaPackageManagerCommands("deno"); err == nil {
		t.Fatal("expected unsupported runtime error")
	}
}
