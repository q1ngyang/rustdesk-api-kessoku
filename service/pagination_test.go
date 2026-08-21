package service

import (
	"strings"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPaginateCapsPageSizeAndRejectsOverflow(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	largeSQL := database.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&model.User{}).Scopes(Paginate(1, 1_000_000)).Find(&[]model.User{})
	})
	if !strings.Contains(largeSQL, "LIMIT 1000") {
		t.Fatalf("uncapped pagination SQL: %s", largeSQL)
	}
	overflowSQL := database.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&model.User{}).Scopes(Paginate(^uint(0), MaxPageSize)).Find(&[]model.User{})
	})
	if !strings.Contains(overflowSQL, "1 = 0") {
		t.Fatalf("overflowing pagination did not fail closed: %s", overflowSQL)
	}
}
