package models

import (
	"testing"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func setupSettingDb(t *testing.T) func() {
	t.Helper()
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	original := Db
	Db = db
	return func() { Db = original }
}

func TestLLM_RoundTrip(t *testing.T) {
	defer setupSettingDb(t)()

	s := new(Setting)

	// 初始为空
	cfg, err := s.LLM()
	if err != nil {
		t.Fatalf("LLM: %v", err)
	}
	if cfg.Enable || cfg.BaseURL != "" || cfg.ApiKey != "" || cfg.Model != "" {
		t.Fatalf("expected empty config, got %+v", cfg)
	}

	// 写入
	if err := s.UpdateLLM(true, "https://api.example.com/v1", "sk-abc", "gpt-x"); err != nil {
		t.Fatalf("UpdateLLM: %v", err)
	}
	cfg, _ = s.LLM()
	if !cfg.Enable || cfg.BaseURL != "https://api.example.com/v1" || cfg.ApiKey != "sk-abc" || cfg.Model != "gpt-x" {
		t.Fatalf("unexpected config after update: %+v", cfg)
	}

	// 再次更新（含 enable=false），应覆盖而非新增
	if err := s.UpdateLLM(false, "https://api2.example.com/v1", "sk-def", "gpt-y"); err != nil {
		t.Fatalf("UpdateLLM 2: %v", err)
	}
	cfg, _ = s.LLM()
	if cfg.Enable || cfg.BaseURL != "https://api2.example.com/v1" || cfg.ApiKey != "sk-def" {
		t.Fatalf("update did not overwrite: %+v", cfg)
	}

	// 确认每个 key 只有一行（更新而非插入新行）
	var count int64
	Db.Model(&Setting{}).Where("code = ?", LLMCode).Count(&count)
	if count != 4 {
		t.Fatalf("expected 4 llm setting rows, got %d", count)
	}
}
