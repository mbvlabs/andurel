package inertia

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestResolveSSRBundlePrefersDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bundle := filepath.Join(dir, "ssr.js")
	if err := os.WriteFile(bundle, []byte("from-disk"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, cleanup, err := resolveSSRBundle(bundle, fstest.MapFS{
		"ssr.js": &fstest.MapFile{Data: []byte("from-embed")},
	})
	if err != nil {
		t.Fatalf("resolveSSRBundle: %v", err)
	}
	if cleanup != nil {
		t.Fatal("expected no temp cleanup when disk file exists")
	}
	if path != bundle {
		t.Fatalf("path = %q, want %q", path, bundle)
	}
}

func TestResolveSSRBundleFallsBackToEmbedFS(t *testing.T) {
	t.Parallel()

	bundleFS := fstest.MapFS{
		"dist/ssr/ssr.js":     &fstest.MapFile{Data: []byte("from-embed")},
		"dist/ssr/ssr.js.map": &fstest.MapFile{Data: []byte(`{"version":3}`)},
	}

	path, cleanup, err := resolveSSRBundle("assets/dist/ssr/ssr.js", bundleFS)
	if err != nil {
		t.Fatalf("resolveSSRBundle: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected temp cleanup")
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "from-embed" {
		t.Fatalf("bundle = %q", data)
	}
	mapData, err := os.ReadFile(path + ".map")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mapData), "version") {
		t.Fatalf("map = %q", mapData)
	}
}

func TestResolveSSRBundleMissingWithoutFS(t *testing.T) {
	t.Parallel()

	_, _, err := resolveSSRBundle(filepath.Join(t.TempDir(), "missing.js"), nil)
	if err == nil {
		t.Fatal("expected missing bundle error")
	}
}

func TestEmbedPathForBundleStripsAssetsPrefix(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"assets/dist/ssr/ssr.js":                 "dist/ssr/ssr.js",
		"dist/ssr/ssr.js":                        "dist/ssr/ssr.js",
		"./dist/ssr/ssr.js":                      "dist/ssr/ssr.js",
		"/srv/app/assets/dist/ssr/ssr.js":        "dist/ssr/ssr.js",
		"/tmp/custom-bundle/ssr.js":              "dist/ssr/ssr.js",
		filepath.Join("custom", "ssr-bundle.js"): "custom/ssr-bundle.js",
	}
	for bundle, want := range tests {
		if got := embedPathForBundle(bundle); got != want {
			t.Fatalf("embedPathForBundle(%q) = %q, want %q", bundle, got, want)
		}
	}
}

func TestResolveSSRBundleFallsBackToEmbedForAbsolutePath(t *testing.T) {
	t.Parallel()

	bundleFS := fstest.MapFS{
		"dist/ssr/ssr.js": &fstest.MapFile{Data: []byte("from-embed")},
	}

	path, cleanup, err := resolveSSRBundle(
		filepath.Join(t.TempDir(), "missing-ssr.js"),
		bundleFS,
	)
	if err != nil {
		t.Fatalf("resolveSSRBundle: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected temp cleanup")
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "from-embed" {
		t.Fatalf("bundle = %q", data)
	}
}

func TestResolveSSRBundleRequiresFileInFS(t *testing.T) {
	t.Parallel()

	_, _, err := resolveSSRBundle("assets/dist/ssr/ssr.js", fstest.MapFS{})
	if err == nil {
		t.Fatal("expected missing embed error")
	}
	for _, want := range []string{"embedded fallback", "dist/ssr/ssr.js", "was tried"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
}
