package service

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Internal service tests cannot import fixtures (fixtures imports service).
// Match its isolated PostgreSQL schema contract, retaining the same schema
// across service reopen. SQLite-specific file/migration tests stay explicit.
func databaseTestConfig(t *testing.T, cfg Config) Config {
	t.Helper()
	if os.Getenv("DRIVEBRIDGE_TEST_DATABASE") != "postgres" || cfg.TeamDSN != "" {
		return cfg
	}
	raw := os.Getenv("DRIVEBRIDGE_TEST_PG_DSN")
	u, err := url.Parse(raw)
	if err != nil || raw == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		t.Fatal("PostgreSQL test DSN is required; SQLite fallback is forbidden")
	}
	sum := sha256.Sum256([]byte(cfg.State))
	schema := fmt.Sprintf("internal_%x", sum[:8])
	query := u.Query()
	query.Set("options", "-c search_path=public")
	u.RawQuery = query.Encode()
	adminDSN := u.String()
	openAdmin := func() *gorm.DB {
		t.Helper()
		db, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal("PostgreSQL test database unavailable")
		}
		return db
	}
	admin := openAdmin()
	conn, err := admin.DB()
	if err != nil {
		t.Fatal("PostgreSQL test connection unavailable")
	}
	// Identifier is generated only from the private fixture path digest.
	err = admin.Exec("CREATE SCHEMA IF NOT EXISTS " + schema).Error
	_ = conn.Close()
	if err != nil {
		t.Fatal("PostgreSQL test schema creation failed")
	}
	t.Cleanup(func() {
		db := openAdmin()
		connection, err := db.DB()
		if err != nil {
			t.Error("PostgreSQL test cleanup connection unavailable")
			return
		}
		defer connection.Close()
		if db.Exec("DROP SCHEMA IF EXISTS "+schema+" CASCADE").Error != nil {
			t.Error("PostgreSQL test schema cleanup failed")
		}
	})
	query.Set("options", "-c search_path="+schema+",public")
	u.RawQuery = query.Encode()
	cfg.TeamDSN = u.String()
	return cfg
}

func TestInternalPersistenceUsesSelectedDatabase(t *testing.T) {
	s, _, _ := blobRig(t)
	expected := "sqlite"
	if os.Getenv("DRIVEBRIDGE_TEST_DATABASE") == "postgres" {
		expected = "postgres"
	}
	if s.DB.Dialector.Name() != expected {
		t.Fatalf("database mismatch: expected %s, got %s", expected, s.DB.Dialector.Name())
	}
	t.Logf("internal_persistence_database=%s", expected)
}
