package ddl

import (
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestForeignKeyParsing(t *testing.T) {
	// Test inline REFERENCES syntax
	sql := `
	CREATE TABLE posts (
		id UUID PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMP DEFAULT NOW()
	)`

	stmt, err := ParseDDLStatement(sql, "test_migration.sql", "postgresql")
	if err != nil {
		t.Fatalf("Failed to parse CREATE TABLE with foreign key: %v", err)
	}

	if stmt.Type != CreateTable {
		t.Fatalf("Expected CREATE TABLE statement, got %v", stmt.Type)
	}

	// Test that the parsing includes foreign keys
	// The parsed statement should have created a table object with foreign keys
	cat := catalog.NewCatalog("public")
	err = ApplyDDL(cat, sql, "test_migration.sql", "postgresql")
	if err != nil {
		t.Fatalf("Failed to apply DDL: %v", err)
	}

	// Verify the table was created with foreign key
	table, err := cat.GetTable("", "posts")
	if err != nil {
		t.Fatalf("Failed to get posts table: %v", err)
	}

	foreignKeys := table.GetForeignKeys()
	if len(foreignKeys) != 1 {
		t.Fatalf("Expected 1 foreign key, got %d", len(foreignKeys))
	}

	fk := foreignKeys[0]
	if fk.Column != "user_id" {
		t.Errorf("Expected foreign key column 'user_id', got '%s'", fk.Column)
	}
	if fk.ReferencedTable != "users" {
		t.Errorf("Expected foreign key to reference 'users', got '%s'", fk.ReferencedTable)
	}
	if fk.ReferencedColumn != "id" {
		t.Errorf("Expected foreign key to reference 'id', got '%s'", fk.ReferencedColumn)
	}
	if fk.OnDelete != catalog.Cascade {
		t.Errorf("Expected CASCADE on delete, got %v", fk.OnDelete)
	}
}

func TestTableLevelForeignKeyParsing(t *testing.T) {
	// Test table-level FOREIGN KEY syntax
	sql := `
	CREATE TABLE posts (
		id UUID PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		user_id UUID NOT NULL,
		category_id UUID,
		created_at TIMESTAMP DEFAULT NOW(),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		CONSTRAINT fk_posts_category FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL
	)`

	cat := catalog.NewCatalog("public")
	err := ApplyDDL(cat, sql, "test_migration.sql", "postgresql")
	if err != nil {
		t.Fatalf("Failed to apply DDL: %v", err)
	}

	// Verify the table was created with foreign keys
	table, err := cat.GetTable("", "posts")
	if err != nil {
		t.Fatalf("Failed to get posts table: %v", err)
	}

	foreignKeys := table.GetForeignKeys()
	if len(foreignKeys) != 2 {
		t.Fatalf("Expected 2 foreign keys, got %d", len(foreignKeys))
	}

	// Check first foreign key (user_id)
	var userFK, categoryFK *catalog.ForeignKey
	for _, fk := range foreignKeys {
		if fk.Column == "user_id" {
			userFK = fk
		} else if fk.Column == "category_id" {
			categoryFK = fk
		}
	}

	if userFK == nil {
		t.Fatal("Expected foreign key for user_id not found")
	}
	if userFK.ReferencedTable != "users" || userFK.ReferencedColumn != "id" {
		t.Errorf("Expected user FK to reference users.id, got %s.%s", userFK.ReferencedTable, userFK.ReferencedColumn)
	}
	if userFK.OnDelete != catalog.Cascade {
		t.Errorf("Expected CASCADE on delete for user FK, got %v", userFK.OnDelete)
	}

	// Check second foreign key (category_id)
	if categoryFK == nil {
		t.Fatal("Expected foreign key for category_id not found")
	}
	if categoryFK.Name != "fk_posts_category" {
		t.Errorf("Expected constraint name 'fk_posts_category', got '%s'", categoryFK.Name)
	}
	if categoryFK.ReferencedTable != "categories" || categoryFK.ReferencedColumn != "id" {
		t.Errorf("Expected category FK to reference categories.id, got %s.%s", categoryFK.ReferencedTable, categoryFK.ReferencedColumn)
	}
	if categoryFK.OnDelete != catalog.SetNull {
		t.Errorf("Expected SET NULL on delete for category FK, got %v", categoryFK.OnDelete)
	}
}

func TestRelationDiscoveryWithParsedForeignKeys(t *testing.T) {
	// Create a catalog with tables that have foreign key relationships via DDL parsing
	cat := catalog.NewCatalog("public")

	// Create users table
	usersSQL := `
	CREATE TABLE users (
		id UUID PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	)`

	err := ApplyDDL(cat, usersSQL, "001_create_users.sql", "postgresql")
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	// Create posts table with foreign key to users
	postsSQL := `
	CREATE TABLE posts (
		id UUID PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		content TEXT,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	)`

	err = ApplyDDL(cat, postsSQL, "002_create_posts.sql", "postgresql")
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}

	// Test relationship discovery
	graph, err := cat.GetRelationshipGraph()
	if err != nil {
		t.Fatalf("Failed to get relationship graph: %v", err)
	}

	// Test users relationships (should have one-to-many to posts)
	usersRelations := graph.GetRelations("users")
	if len(usersRelations) != 1 {
		t.Fatalf("Expected 1 relation for users, got %d", len(usersRelations))
	}

	userRel := usersRelations[0]
	if userRel.Type != catalog.OneToMany {
		t.Errorf("Expected OneToMany relation for users, got %s", userRel.Type)
	}
	if userRel.ToTable != "posts" {
		t.Errorf("Expected relation to posts, got %s", userRel.ToTable)
	}

	// Test posts relationships (should have many-to-one to users)
	postsRelations := graph.GetRelations("posts")
	if len(postsRelations) != 1 {
		t.Fatalf("Expected 1 relation for posts, got %d", len(postsRelations))
	}

	postRel := postsRelations[0]
	if postRel.Type != catalog.ManyToOne {
		t.Errorf("Expected ManyToOne relation for posts, got %s", postRel.Type)
	}
	if postRel.ToTable != "users" {
		t.Errorf("Expected relation to users, got %s", postRel.ToTable)
	}

	t.Logf("SUCCESS: Foreign keys parsed from DDL and relations discovered correctly")
	t.Logf("  - Users has OneToMany relation to posts")
	t.Logf("  - Posts has ManyToOne relation to users")
}