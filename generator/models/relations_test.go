package models

import (
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestRelationDiscovery(t *testing.T) {
	// Create a test catalog with relations
	cat := catalog.NewCatalog("public")

	// Create users table
	usersTable := catalog.NewTable("", "users")
	usersTable.AddColumn(catalog.NewColumn("id", "uuid").SetPrimaryKey())
	usersTable.AddColumn(catalog.NewColumn("name", "varchar"))
	usersTable.AddColumn(catalog.NewColumn("email", "varchar"))
	usersTable.AddColumn(catalog.NewColumn("created_at", "timestamp"))
	usersTable.AddColumn(catalog.NewColumn("updated_at", "timestamp"))

	// Create posts table with foreign key to users
	postsTable := catalog.NewTable("", "posts")
	postsTable.AddColumn(catalog.NewColumn("id", "uuid").SetPrimaryKey())
	postsTable.AddColumn(catalog.NewColumn("title", "varchar"))
	postsTable.AddColumn(catalog.NewColumn("content", "text"))
	postsTable.AddColumn(catalog.NewColumn("user_id", "uuid"))
	postsTable.AddColumn(catalog.NewColumn("created_at", "timestamp"))
	postsTable.AddColumn(catalog.NewColumn("updated_at", "timestamp"))

	// Add foreign key from posts to users
	fk := catalog.NewForeignKey("fk_posts_user", "user_id", "users", "id")
	fk.SetOnDelete(catalog.Cascade)
	postsTable.AddForeignKey(fk)

	// Add tables to catalog
	cat.AddTable("", usersTable)
	cat.AddTable("", postsTable)

	// Create model generator
	generator := NewGenerator("postgresql")

	// Test generating User model with relations
	config := Config{
		TableName:    "users",
		ResourceName: "User",
		PackageName:  "models",
		DatabaseType: "postgresql",
		ModulePath:   "github.com/test/app",
	}

	model, err := generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build User model: %v", err)
	}

	t.Logf("Generated User model with %d relations", len(model.Relations))
	for _, rel := range model.Relations {
		t.Logf("  - %s: %s %s -> %s.%s", 
			rel.Name, rel.Type, rel.ForeignKey, rel.RelatedTable, rel.RelatedColumn)
	}

	// User should have one one-to-many relation to posts
	if len(model.Relations) != 1 {
		t.Errorf("Expected 1 relation for User, got %d", len(model.Relations))
	}

	if len(model.Relations) > 0 {
		rel := model.Relations[0]
		if rel.Type != catalog.OneToMany {
			t.Errorf("Expected OneToMany relation, got %s", rel.Type)
		}
		if rel.RelatedTable != "posts" {
			t.Errorf("Expected relation to posts, got %s", rel.RelatedTable)
		}
	}

	// Test generating Post model with relations
	config.TableName = "posts"
	config.ResourceName = "Post"
	
	model, err = generator.Build(cat, config)
	if err != nil {
		t.Fatalf("Failed to build Post model: %v", err)
	}

	t.Logf("Generated Post model with %d relations", len(model.Relations))
	for _, rel := range model.Relations {
		t.Logf("  - %s: %s %s -> %s.%s", 
			rel.Name, rel.Type, rel.ForeignKey, rel.RelatedTable, rel.RelatedColumn)
	}

	// Post should have one many-to-one relation to users
	if len(model.Relations) != 1 {
		t.Errorf("Expected 1 relation for Post, got %d", len(model.Relations))
	}

	if len(model.Relations) > 0 {
		rel := model.Relations[0]
		if rel.Type != catalog.ManyToOne {
			t.Errorf("Expected ManyToOne relation, got %s", rel.Type)
		}
		if rel.RelatedTable != "users" {
			t.Errorf("Expected relation to users, got %s", rel.RelatedTable)
		}
	}
}