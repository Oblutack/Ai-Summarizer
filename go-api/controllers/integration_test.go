package controllers

// Integration tests against a real Postgres. They run only when TEST_DSN is set, e.g.
//
//	TEST_DSN="host=localhost port=5433 user=user password=... dbname=summarizer_test sslmode=disable"
//
// They DROP the public schema, so the database name must contain "test".

import (
	"ai-summarizer/go-api/initializers"
	"ai-summarizer/go-api/models"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN not set; skipping Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	var name string
	if err := db.Raw("SELECT current_database()").Scan(&name).Error; err != nil || !strings.Contains(name, "test") {
		t.Fatalf("refusing to wipe database %q: its name must contain \"test\"", name)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
		t.Fatal(err)
	}
	initializers.DB = db
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	return db
}

func migrated(t *testing.T) *gorm.DB {
	t.Helper()
	db := testDB(t)
	if err := initializers.RunMigrations(); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

func scalar[T any](t *testing.T, db *gorm.DB, query string, args ...any) T {
	t.Helper()
	var out T
	if err := db.Raw(query, args...).Scan(&out).Error; err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return out
}

// ---- migrations ----------------------------------------------------------------------------

func TestMigrationsBuildAFreshDatabase(t *testing.T) {
	db := migrated(t)

	for _, table := range []string{"users", "documents"} {
		if scalar[int](t, db, "SELECT count(*) FROM information_schema.tables WHERE table_name = ?", table) != 1 {
			t.Errorf("table %s missing", table)
		}
	}
	if scalar[int](t, db, "SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_documents_user_id_id'") != 1 {
		t.Error("the user/id index is missing")
	}
	if scalar[string](t, db, "SELECT is_nullable FROM information_schema.columns WHERE table_name='documents' AND column_name='has_content'") != "NO" {
		t.Error("has_content should be NOT NULL")
	}
	if scalar[int](t, db, "SELECT count(*) FROM pg_constraint WHERE conname = 'uni_users_email'") != 1 {
		t.Error("the unique email constraint is missing")
	}

	// the models and the schema agree, so GORM can use the migrated tables as-is
	user := models.User{Email: "a@example.com", Password: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Document{Filename: "f", Summary: "s", UserID: user.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.User{Email: "a@example.com", Password: "y"}).Error; err == nil {
		t.Error("duplicate emails must be rejected by the database")
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	migrated(t)
	if err := initializers.RunMigrations(); err != nil {
		t.Fatalf("second run should be a no-op: %v", err)
	}
}

func TestMigrationsUpgradeALegacyDatabase(t *testing.T) {
	db := testDB(t)

	// The schema the old AutoMigrate produced before documents had content/has_content.
	legacy := []string{
		`CREATE TABLE users (id BIGSERIAL PRIMARY KEY, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ, email TEXT, password TEXT, CONSTRAINT uni_users_email UNIQUE (email))`,
		`CREATE TABLE documents (id BIGSERIAL PRIMARY KEY, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ, filename TEXT, summary TEXT, user_id BIGINT)`,
		`INSERT INTO users (email, password) VALUES ('old@example.com', 'hash')`,
		`INSERT INTO documents (filename, summary, user_id) VALUES ('old.pdf', 'old summary', 1)`,
	}
	for _, q := range legacy {
		if err := db.Exec(q).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	if err := initializers.RunMigrations(); err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	var doc models.Document
	if err := db.First(&doc).Error; err != nil {
		t.Fatalf("existing data must survive: %v", err)
	}
	if doc.Filename != "old.pdf" || doc.Summary != "old summary" {
		t.Errorf("data changed: %+v", doc)
	}
	if doc.HasContent {
		t.Error("legacy documents have no stored text, so has_content must be false")
	}
	if scalar[int](t, db, "SELECT count(*) FROM users WHERE email='old@example.com'") != 1 {
		t.Error("existing users must survive")
	}
}

// ---- pagination ----------------------------------------------------------------------------

func listRouter(userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user", models.User{Model: gorm.Model{ID: userID}}) })
	r.GET("/documents", ListDocuments)
	r.DELETE("/documents/:id", DeleteDocument)
	r.POST("/documents/:id/chat", ChatWithDocument)
	return r
}

func listPage(t *testing.T, r *gin.Engine, query string) ([]models.Document, string, int) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/documents"+query, nil))
	var docs []models.Document
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &docs); err != nil {
			t.Fatalf("bad JSON: %v\n%s", err, w.Body.String())
		}
	}
	return docs, w.Header().Get(NextCursorHeader), w.Code
}

func seedDocuments(t *testing.T, db *gorm.DB, userID uint, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		d := models.Document{Filename: fmt.Sprintf("doc-%d", i), Summary: "s", UserID: userID, Content: "source text", HasContent: true}
		if err := db.Create(&d).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestListDocumentsPagesNewestFirstWithoutGapsOrDuplicates(t *testing.T) {
	db := migrated(t)
	seedDocuments(t, db, 1, 45)
	seedDocuments(t, db, 2, 5) // another user's documents must never appear
	r := listRouter(1)

	var all []models.Document
	cursor, pages := "", 0
	for {
		q := "?limit=20"
		if cursor != "" {
			q += "&before=" + cursor
		}
		docs, next, code := listPage(t, r, q)
		if code != http.StatusOK {
			t.Fatalf("page %d: HTTP %d", pages, code)
		}
		all = append(all, docs...)
		pages++
		if next == "" {
			break
		}
		cursor = next
		if pages > 10 {
			t.Fatal("pagination never ended")
		}
	}

	if len(all) != 45 || pages != 3 {
		t.Fatalf("got %d documents over %d pages, want 45 over 3", len(all), pages)
	}
	seen := map[uint]bool{}
	for i, d := range all {
		if d.UserID != 1 {
			t.Fatalf("leaked another user's document: %+v", d)
		}
		if seen[d.ID] {
			t.Fatalf("duplicate document %d", d.ID)
		}
		seen[d.ID] = true
		if i > 0 && all[i-1].ID <= d.ID {
			t.Fatalf("not newest-first at position %d", i)
		}
		if d.Content != "" {
			t.Fatal("the list must not load document content")
		}
	}
}

func TestListDocumentsEdgeCases(t *testing.T) {
	db := migrated(t)
	seedDocuments(t, db, 1, 3)
	r := listRouter(1)

	docs, next, code := listPage(t, r, "")
	if code != 200 || len(docs) != 3 || next != "" {
		t.Errorf("default page: %d docs, next %q, HTTP %d", len(docs), next, code)
	}

	docs, next, _ = listPage(t, r, "?limit=3")
	if len(docs) != 3 || next != "" {
		t.Errorf("a page that exactly fits must not claim there is more: %d docs, next %q", len(docs), next)
	}

	docs, next, _ = listPage(t, r, "?limit=2")
	if len(docs) != 2 || next == "" {
		t.Errorf("limit=2: %d docs, next %q", len(docs), next)
	}

	if _, _, code := listPage(t, r, "?limit=1000"); code != 200 {
		t.Errorf("an oversized limit should be clamped, got HTTP %d", code)
	}
	for _, q := range []string{"?limit=0", "?limit=-5", "?limit=abc", "?before=xyz", "?before=-1"} {
		if _, _, code := listPage(t, r, q); code != http.StatusBadRequest {
			t.Errorf("%s: HTTP %d, want 400", q, code)
		}
	}

	docs, _, _ = listPage(t, listRouter(999), "")
	if len(docs) != 0 {
		t.Errorf("a user with no documents got %d", len(docs))
	}
}

func TestDeletedDocumentsDisappearFromTheList(t *testing.T) {
	db := migrated(t)
	seedDocuments(t, db, 1, 3)
	r := listRouter(1)

	docs, _, _ := listPage(t, r, "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/documents/%d", docs[0].ID), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("delete: HTTP %d %s", w.Code, w.Body.String())
	}
	if after, _, _ := listPage(t, r, ""); len(after) != 2 {
		t.Errorf("got %d documents after a delete, want 2", len(after))
	}
}

func TestDocumentOwnershipIsEnforced(t *testing.T) {
	db := migrated(t)
	seedDocuments(t, db, 1, 1)
	var doc models.Document
	db.First(&doc)
	stranger := listRouter(2)

	w := httptest.NewRecorder()
	stranger.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/documents/%d", doc.ID), nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("deleting someone else's document: HTTP %d, want 403", w.Code)
	}

	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/documents/%d/chat", doc.ID), strings.NewReader(`{"question":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	stranger.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("chatting with someone else's document: HTTP %d, want 404", w.Code)
	}
}

func TestChatOnALegacyDocumentIsAConflict(t *testing.T) {
	db := migrated(t)
	if err := db.Create(&models.Document{Filename: "legacy.pdf", Summary: "s", UserID: 1}).Error; err != nil {
		t.Fatal(err)
	}
	var doc models.Document
	db.First(&doc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/documents/%d/chat", doc.ID), strings.NewReader(`{"question":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	listRouter(1).ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("HTTP %d, want 409", w.Code)
	}
}
