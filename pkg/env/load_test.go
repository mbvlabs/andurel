package env

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
		wantErr string
	}{
		{
			name:    "simple",
			content: "KEY=value",
			want:    map[string]string{"KEY": "value"},
		},
		{
			name:    "blank lines and comments",
			content: "\n# comment\n  # indented comment\n\nKEY=value\n",
			want:    map[string]string{"KEY": "value"},
		},
		{
			name:    "export prefix",
			content: "export KEY=value\nexport\tTAB=two\n",
			want:    map[string]string{"KEY": "value", "TAB": "two"},
		},
		{
			name:    "surrounding whitespace",
			content: "  KEY = value  \n",
			want:    map[string]string{"KEY": "value"},
		},
		{
			name:    "empty value",
			content: "EMPTY=\n",
			want:    map[string]string{"EMPTY": ""},
		},
		{
			name:    "single quotes literal",
			content: `KEY='a # b \n "c"'` + "\n",
			want:    map[string]string{"KEY": `a # b \n "c"`},
		},
		{
			name:    "double quote escapes",
			content: "KEY=\"a\\nb\\\"c\\\\d\"\n",
			want:    map[string]string{"KEY": "a\nb\"c\\d"},
		},
		{
			name:    "double quote unknown escape kept",
			content: "KEY=\"a\\tb\"\n",
			want:    map[string]string{"KEY": `a\tb`},
		},
		{
			name:    "quoted value with trailing comment",
			content: "KEY=\"value\" # comment\n",
			want:    map[string]string{"KEY": "value"},
		},
		{
			name:    "inline comment",
			content: "KEY=value # comment\n",
			want:    map[string]string{"KEY": "value"},
		},
		{
			name:    "hash without leading space is part of value",
			content: "KEY=value#not-a-comment\n",
			want:    map[string]string{"KEY": "value#not-a-comment"},
		},
		{
			name:    "carriage returns",
			content: "KEY=value\r\nOTHER=two\r\n",
			want:    map[string]string{"KEY": "value", "OTHER": "two"},
		},
		{
			name:    "later duplicate wins",
			content: "KEY=one\nKEY=two\n",
			want:    map[string]string{"KEY": "two"},
		},
		{
			name:    "missing equals",
			content: "KEY value\n",
			wantErr: "test.env:1: expected KEY=VALUE",
		},
		{
			name:    "missing equals reports line",
			content: "\n\nBROKEN\n",
			wantErr: "test.env:3: expected KEY=VALUE",
		},
		{
			name:    "empty key",
			content: "=value\n",
			wantErr: "test.env:1: empty key",
		},
		{
			name:    "unterminated single quote",
			content: "KEY='value\n",
			wantErr: "test.env:1: unterminated quoted value",
		},
		{
			name:    "unterminated double quote",
			content: "KEY=\"value\n",
			wantErr: "test.env:1: unterminated quoted value",
		},
		{
			name:    "text after quoted value",
			content: "KEY=\"value\" extra\n",
			wantErr: `test.env:1: unexpected "extra" after quoted value`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parse("test.env", []byte(test.content))
			if test.wantErr != "" {
				if err == nil || err.Error() != test.wantErr {
					t.Fatalf("parse() error = %v, want %q", err, test.wantErr)
				}

				return
			}
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}
			if !maps.Equal(got, test.want) {
				t.Fatalf("parse() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestLoadDefaultMissingIsNotAnError(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadExplicitMissingIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.env")

	err := Load(path)

	if err == nil {
		t.Fatal("Load() error = nil, want a missing file error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load() error = %v, want fs.ErrNotExist", err)
	}
	if !strings.HasPrefix(err.Error(), "env: load: ") {
		t.Fatalf("Load() error = %q, want an env: load prefix", err)
	}
}

func TestLoadSetsUnsetKeysAndKeepsProcessValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "ANDUREL_ENV_TEST_NEW=from-file\n" +
		"ANDUREL_ENV_TEST_EXISTING=from-file\n" +
		"ANDUREL_ENV_TEST_EMPTY=\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	clearEnv(t, "ANDUREL_ENV_TEST_NEW", "ANDUREL_ENV_TEST_EXISTING", "ANDUREL_ENV_TEST_EMPTY")
	t.Setenv("ANDUREL_ENV_TEST_EXISTING", "from-process")

	if err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := os.Getenv("ANDUREL_ENV_TEST_NEW"); got != "from-file" {
		t.Fatalf("NEW = %q, want %q", got, "from-file")
	}
	if got := os.Getenv("ANDUREL_ENV_TEST_EXISTING"); got != "from-process" {
		t.Fatalf("EXISTING = %q, want %q", got, "from-process")
	}
	if value, ok := os.LookupEnv("ANDUREL_ENV_TEST_EMPTY"); !ok || value != "" {
		t.Fatalf("EMPTY = %q (set %v), want an empty set value", value, ok)
	}
}

func TestLoadDoesNotOverrideAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.env")
	second := filepath.Join(dir, "second.env")
	if err := os.WriteFile(first, []byte("ANDUREL_ENV_TEST_SHARED=first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content := "ANDUREL_ENV_TEST_SHARED=second\nANDUREL_ENV_TEST_LATER=second\n"
	if err := os.WriteFile(second, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	clearEnv(t, "ANDUREL_ENV_TEST_SHARED", "ANDUREL_ENV_TEST_LATER")

	if err := Load(first, second); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := os.Getenv("ANDUREL_ENV_TEST_SHARED"); got != "first" {
		t.Fatalf("SHARED = %q, want %q", got, "first")
	}
	if got := os.Getenv("ANDUREL_ENV_TEST_LATER"); got != "second" {
		t.Fatalf("LATER = %q, want %q", got, "second")
	}
}

func TestLoadDefaultFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(".env", []byte("ANDUREL_ENV_TEST_DEFAULT=loaded\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	clearEnv(t, "ANDUREL_ENV_TEST_DEFAULT")

	if err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := os.Getenv("ANDUREL_ENV_TEST_DEFAULT"); got != "loaded" {
		t.Fatalf("DEFAULT = %q, want %q", got, "loaded")
	}
}

func TestLoadParseErrorNamesFileAndLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.env")
	if err := os.WriteFile(path, []byte("\nBROKEN\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Load(path)

	if err == nil {
		t.Fatal("Load() error = nil, want a parse error")
	}
	if !strings.HasPrefix(err.Error(), "env: load: ") {
		t.Fatalf("Load() error = %q, want an env: load prefix", err)
	}
	if !strings.Contains(err.Error(), path+":2: expected KEY=VALUE") {
		t.Fatalf("Load() error = %q, want the file and line", err)
	}
}

func clearEnv(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		previous, ok := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(key, previous)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}
