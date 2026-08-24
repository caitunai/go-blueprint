package db

import (
	"database/sql"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestCloseDisconnectsDatabasePool(t *testing.T) {
	sqlDB, err := sql.Open("mysql", "user:password@tcp(127.0.0.1:1)/test")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("sql.DB.Close() cleanup error = %v", err)
		}
	})
	database, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	dbMutex.Lock()
	db = database
	dbMutex.Unlock()

	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("Ping() succeeded after Close()")
	}
	if err := Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}
