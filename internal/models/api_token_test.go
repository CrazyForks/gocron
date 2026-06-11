package models

import (
	"testing"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func setupApiTokenTestDb(t *testing.T) func() {
	t.Helper()
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ApiToken{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	originalDb := Db
	Db = db
	return func() { Db = originalDb }
}

func TestHashToken_Deterministic(t *testing.T) {
	a := HashToken("gcx_secret")
	b := HashToken("gcx_secret")
	if a != b {
		t.Fatalf("expected deterministic hash, got %q and %q", a, b)
	}
	if a == HashToken("gcx_other") {
		t.Fatal("expected different hashes for different inputs")
	}
	if len(a) != 64 {
		t.Fatalf("expected sha256 hex of length 64, got %d", len(a))
	}
}

func TestApiToken_CreateAndFindByHash(t *testing.T) {
	defer setupApiTokenTestDb(t)()

	hash := HashToken("gcx_plaintext")
	token := &ApiToken{UserId: 1, Name: "test", TokenHash: hash}
	if err := token.Create(); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if token.Id == 0 {
		t.Fatal("expected auto-assigned id")
	}

	found := &ApiToken{}
	if err := found.FindByHash(hash); err != nil {
		t.Fatalf("find by hash failed: %v", err)
	}
	if found.Id != token.Id || found.UserId != 1 {
		t.Fatalf("found wrong token: %+v", found)
	}

	missing := &ApiToken{}
	if err := missing.FindByHash(HashToken("nope")); err == nil {
		t.Fatal("expected error finding non-existent hash")
	}
}

func TestApiToken_ListByUser(t *testing.T) {
	defer setupApiTokenTestDb(t)()

	for i := range 3 {
		token := &ApiToken{UserId: 1, Name: "u1", TokenHash: HashToken(string(rune('a' + i)))}
		if err := token.Create(); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}
	other := &ApiToken{UserId: 2, Name: "u2", TokenHash: HashToken("z")}
	if err := other.Create(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	list, err := (&ApiToken{}).ListByUser(1)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 tokens for user 1, got %d", len(list))
	}
	// 倒序：最后创建的 id 最大，应排第一
	if list[0].Id < list[len(list)-1].Id {
		t.Fatal("expected descending order by id")
	}
}

func TestApiToken_DeleteScopedToUser(t *testing.T) {
	defer setupApiTokenTestDb(t)()

	token := &ApiToken{UserId: 1, Name: "mine", TokenHash: HashToken("mine")}
	if err := token.Create(); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// 其他用户尝试删除：不应影响
	affected, err := (&ApiToken{}).Delete(token.Id, 999)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if affected != 0 {
		t.Fatalf("expected 0 rows affected for cross-user delete, got %d", affected)
	}

	// 拥有者删除：成功
	affected, err = (&ApiToken{}).Delete(token.Id, 1)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 row affected, got %d", affected)
	}
}

func TestApiToken_TouchLastUsed(t *testing.T) {
	defer setupApiTokenTestDb(t)()

	token := &ApiToken{UserId: 1, Name: "t", TokenHash: HashToken("t")}
	if err := token.Create(); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if token.LastUsedAt != nil {
		t.Fatal("expected nil LastUsedAt on create")
	}

	token.TouchLastUsed()

	reloaded := &ApiToken{}
	if err := reloaded.FindByHash(HashToken("t")); err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if reloaded.LastUsedAt == nil {
		t.Fatal("expected LastUsedAt to be set after TouchLastUsed")
	}
}
