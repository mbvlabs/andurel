package layout

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedFlashSessionNameIsValidCookieName(t *testing.T) {
	flash := readGeneratedApplicationTemplate(t, "router_cookies_flash.tmpl")
	for _, want := range []string{
		`"github.com/gosimple/slug"`,
		`base := slug.Make(strings.ToLower(projectName))`,
		`return base + "_flash_key"`,
		`return base + "_dev_flash_key"`,
	} {
		if !strings.Contains(flash, want) {
			t.Errorf("router_cookies_flash.tmpl missing %q", want)
		}
	}
	if strings.Contains(flash, `strings.ToLower(projectName) + "_" + "flash_key"`) ||
		strings.Contains(flash, `strings.ToLower(projectName) + "_" + "dev_flash_key"`) {
		t.Error("router_cookies_flash.tmpl still builds cookie names without slugifying projectName")
	}

	nameFn, err := extractGeneratedFunction(flash, "buildFlashSessionName")
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	cookiesDir := filepath.Join(root, "cookies")
	if err := os.MkdirAll(cookiesDir, 0o755); err != nil {
		t.Fatalf("create cookies directory: %v", err)
	}

	serverPath, err := filepath.Abs(filepath.Join("..", "pkg", "server"))
	if err != nil {
		t.Fatal(err)
	}

	goMod := fmt.Sprintf(`module flashcookie

go %s

require (
	github.com/gosimple/slug v1.15.0
	github.com/mbvlabs/andurel/pkg/server v0.0.0
)

replace github.com/mbvlabs/andurel/pkg/server => %q
`, goVersion, filepath.ToSlash(serverPath))

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	source := "package cookies\n\n" +
		"import (\n" +
		"\t\"strings\"\n\n" +
		"\t\"github.com/gosimple/slug\"\n" +
		"\t\"github.com/mbvlabs/andurel/pkg/server\"\n" +
		")\n\n" +
		nameFn + "\n"
	if err := os.WriteFile(filepath.Join(cookiesDir, "flash_name.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(cookiesDir, "flash_name_test.go"),
		[]byte(generatedFlashSessionNameTests),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "test", "-mod=mod", "./cookies")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(root, ".gocache"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated flash cookie name tests failed: %v\n%s", err, output)
	}
}

func extractGeneratedFunction(src, name string) (string, error) {
	sig := "func " + name
	start := strings.Index(src, sig)
	if start < 0 {
		return "", fmt.Errorf("%s not found", name)
	}

	brace := strings.Index(src[start:], "{")
	if brace < 0 {
		return "", fmt.Errorf("%s missing body", name)
	}

	depth := 0
	for i := start + brace; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start : i+1], nil
			}
		}
	}

	return "", fmt.Errorf("%s is unclosed", name)
}

const generatedFlashSessionNameTests = `package cookies

import (
	"net/http"
	"testing"

	"github.com/mbvlabs/andurel/pkg/server"
)

func TestBuildFlashSessionName(t *testing.T) {
	unslugged := "andurel site_dev_flash_key"
	if err := (&http.Cookie{Name: unslugged, Value: "x"}).Valid(); err == nil {
		t.Fatalf("expected %q to be an invalid cookie name", unslugged)
	}

	tests := []struct {
		environment string
		want        string
	}{
		{"development", "andurel-site_dev_flash_key"},
		{server.ProdEnvironment, "andurel-site_flash_key"},
	}
	for _, tc := range tests {
		name := buildFlashSessionName("Andurel Site", tc.environment)
		if name != tc.want {
			t.Errorf(
				"buildFlashSessionName(%q, %q) = %q, want %q",
				"Andurel Site",
				tc.environment,
				name,
				tc.want,
			)
		}
		if err := (&http.Cookie{Name: name, Value: "x"}).Valid(); err != nil {
			t.Errorf("cookie name %q is invalid: %v", name, err)
		}
	}
}
`
