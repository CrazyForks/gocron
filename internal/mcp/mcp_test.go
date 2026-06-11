package mcp

import (
	"testing"

	"github.com/gocronx-team/gocron/internal/models"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupTestDb(t *testing.T) func() {
	t.Helper()
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Task{}, &models.TaskLog{}, &models.Host{}, &models.TaskHost{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	original := models.Db
	models.Db = db
	return func() { models.Db = original }
}

// TestNewServerForUserBuilds 确保按用户构建 MCP server（含全部工具注册与 JSON Schema 推断）
// 不会 panic。AddTool 会从 Go 类型与 jsonschema 标签推断 Schema，标签若含非法字符会在此 panic，
// 而这一步只在真实 MCP 请求触发 getServer 时才跑，请求级测试覆盖不到。
func TestNewServerForUserBuilds(t *testing.T) {
	for _, admin := range []bool{true, false} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("newServerForUser(admin=%v) panicked: %v", admin, r)
				}
			}()
			if s := newServerForUser(&authUser{Id: 1, Name: "a", IsAdmin: admin}); s == nil {
				t.Fatal("expected non-nil server")
			}
		}()
	}
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		header  string
		wantTok string
		wantOk  bool
	}{
		{"Bearer gcx_abc", "gcx_abc", true},
		{"bearer gcx_abc", "gcx_abc", true}, // case-insensitive scheme
		{"  Bearer   gcx_abc  ", "gcx_abc", true},
		{"Bearer ", "", false},
		{"Bearer", "", false},
		{"Token gcx_abc", "", false},
		{"", "", false},
		{"gcx_abc", "", false},
	}
	for _, c := range cases {
		tok, ok := bearerToken(c.header)
		if ok != c.wantOk || tok != c.wantTok {
			t.Errorf("bearerToken(%q) = (%q, %v), want (%q, %v)", c.header, tok, ok, c.wantTok, c.wantOk)
		}
	}
}

func TestNormalizePagination(t *testing.T) {
	if normalizePage(0) != 1 || normalizePage(-3) != 1 {
		t.Error("non-positive page should normalize to 1")
	}
	if normalizePage(5) != 5 {
		t.Error("positive page should pass through")
	}
	if normalizePageSize(0) != 20 {
		t.Error("zero page size should default to 20")
	}
	if normalizePageSize(9999) != maxPageSize {
		t.Errorf("oversized page size should cap at %d", maxPageSize)
	}
	if normalizePageSize(50) != 50 {
		t.Error("in-range page size should pass through")
	}
}

func seedTask(t *testing.T, name string, status models.Status) models.Task {
	t.Helper()
	task := models.Task{
		Name:     name,
		Level:    models.TaskLevelParent,
		Spec:     "* * * * * *",
		Protocol: models.TaskHTTP,
		Command:  "https://example.com",
		Status:   status,
	}
	if err := models.Db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return task
}

func TestListTasks(t *testing.T) {
	defer setupTestDb(t)()

	seedTask(t, "backup-db", models.Enabled)
	seedTask(t, "cleanup-logs", models.Disabled)

	// 全部
	out, err := listTasks(listTasksInput{})
	if err != nil {
		t.Fatalf("listTasks: %v", err)
	}
	if out.Total != 2 || len(out.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got total=%d len=%d", out.Total, len(out.Tasks))
	}

	// 按名称过滤
	out, err = listTasks(listTasksInput{Name: "backup"})
	if err != nil {
		t.Fatalf("listTasks(name): %v", err)
	}
	if out.Total != 1 || out.Tasks[0].Name != "backup-db" {
		t.Fatalf("name filter failed: %+v", out)
	}

	// 按状态过滤：禁用
	disabled := 0
	out, err = listTasks(listTasksInput{Status: &disabled})
	if err != nil {
		t.Fatalf("listTasks(status): %v", err)
	}
	if out.Total != 1 || out.Tasks[0].Name != "cleanup-logs" {
		t.Fatalf("status filter failed: %+v", out)
	}
}

func TestGetTask(t *testing.T) {
	defer setupTestDb(t)()

	created := seedTask(t, "report", models.Enabled)

	got, err := getTask(getTaskInput{Id: created.Id})
	if err != nil {
		t.Fatalf("getTask: %v", err)
	}
	if got.Id != created.Id || got.Name != "report" {
		t.Fatalf("unexpected task: %+v", got)
	}

	// 不存在的 ID：返回空 task，不报错
	missing, err := getTask(getTaskInput{Id: 9999})
	if err != nil {
		t.Fatalf("getTask(missing): %v", err)
	}
	if missing.Id != 0 {
		t.Fatalf("expected empty task for missing id, got %+v", missing)
	}
}

func TestQueryTaskLogs(t *testing.T) {
	defer setupTestDb(t)()

	logs := []models.TaskLog{
		{TaskId: 1, Name: "a", Status: models.Enabled},
		{TaskId: 1, Name: "a", Status: models.Disabled},
		{TaskId: 2, Name: "b", Status: models.Enabled},
	}
	for i := range logs {
		if err := models.Db.Create(&logs[i]).Error; err != nil {
			t.Fatalf("seed log: %v", err)
		}
	}

	out, err := queryTaskLogs(queryTaskLogsInput{TaskId: 1})
	if err != nil {
		t.Fatalf("queryTaskLogs: %v", err)
	}
	if out.Total != 2 || len(out.Logs) != 2 {
		t.Fatalf("expected 2 logs for task 1, got total=%d len=%d", out.Total, len(out.Logs))
	}
}

func TestListHosts(t *testing.T) {
	defer setupTestDb(t)()

	if err := models.Db.Create(&models.Host{Name: "node-1", Port: 5921}).Error; err != nil {
		t.Fatalf("seed host: %v", err)
	}

	out, err := listHosts()
	if err != nil {
		t.Fatalf("listHosts: %v", err)
	}
	if len(out.Hosts) != 1 || out.Hosts[0].Name != "node-1" {
		t.Fatalf("unexpected hosts: %+v", out.Hosts)
	}
}

func TestRunTaskNotFound(t *testing.T) {
	defer setupTestDb(t)()

	// 不存在的任务：返回 Started=false，且不触发实际执行
	out, err := runTask(runTaskInput{Id: 9999})
	if err != nil {
		t.Fatalf("runTask: %v", err)
	}
	if out.Started {
		t.Fatal("expected Started=false for non-existent task")
	}
}
