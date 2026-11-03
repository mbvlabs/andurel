package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateLegacyEmailImportsRewritesUnaliasedSelectors(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) string {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}

		return path
	}

	unaliasedPath := write("services/registration.go", `package services

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/email"
)

var data email.TransactionalData
var sender email.TransactionalSender

func send(ctx context.Context) error {
	return email.SendTransactional(ctx, data, sender)
}

type localEmail struct {
	TransactionalData string
}

func shadow() {
	email := "person@example.com"
	_ = email
}

func shadowSelector() {
	email := localEmail{}
	_ = email.TransactionalData
}
`)

	aliasedPath := write("queue/workers.go", `package queue

import pkgemail "github.com/mbvlabs/andurel/pkg/email"

var sender pkgemail.TransactionalSender

func use() string {
	text, _ := pkgemail.HTMLToText("<p>x</p>")
	return text
}
`)

	explicitEmailAliasPath := write("email/verify_email.go", `package email

import email "github.com/mbvlabs/andurel/pkg/email"

var _ email.Transformer
`)

	migrateLegacyEmailImports(t, root)

	unaliased := readMigrationTestFile(t, unaliasedPath)
	for _, want := range []string{
		`"github.com/mbvlabs/andurel/pkg/postmand"`,
		"postmand.TransactionalData",
		"postmand.TransactionalSender",
		"postmand.SendTransactional",
		`email := "person@example.com"`,
		"_ = email",
		"_ = email.TransactionalData",
	} {
		if !strings.Contains(unaliased, want) {
			t.Errorf("unaliased import migration missing %q:\n%s", want, unaliased)
		}
	}
	for _, unwanted := range []string{
		"github.com/mbvlabs/andurel/pkg/email",
		"var data email.TransactionalData",
		"var sender email.TransactionalSender",
		"email.SendTransactional(ctx",
		"postmand := \"person@example.com\"",
	} {
		if strings.Contains(unaliased, unwanted) {
			t.Errorf("unaliased import migration kept or over-rewrote %q:\n%s", unwanted, unaliased)
		}
	}

	aliased := readMigrationTestFile(t, aliasedPath)
	for _, want := range []string{
		`pkgemail "github.com/mbvlabs/andurel/pkg/postmand"`,
		"pkgemail.TransactionalSender",
		"pkgemail.HTMLToText",
	} {
		if !strings.Contains(aliased, want) {
			t.Errorf("aliased import migration missing %q:\n%s", want, aliased)
		}
	}
	if strings.Contains(aliased, "postmand.TransactionalSender") {
		t.Errorf("aliased import migration rewrote the alias selector:\n%s", aliased)
	}

	explicitEmailAlias := readMigrationTestFile(t, explicitEmailAliasPath)
	for _, want := range []string{
		`email "github.com/mbvlabs/andurel/pkg/postmand"`,
		"email.Transformer",
	} {
		if !strings.Contains(explicitEmailAlias, want) {
			t.Errorf("explicit email alias migration missing %q:\n%s", want, explicitEmailAlias)
		}
	}
	if strings.Contains(explicitEmailAlias, "postmand.Transformer") {
		t.Errorf("explicit email alias migration rewrote the alias selector:\n%s", explicitEmailAlias)
	}
}

func readMigrationTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(content)
}
