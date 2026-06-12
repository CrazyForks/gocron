package main

import (
	"strings"
	"testing"
	"time"

	"github.com/gocronx-team/gocron/internal/models"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// newTaskTestDB 用内存 sqlite + 单连接 + SingularTable（与服务端 CreateDb 命名一致）。
func newTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.Task{}, &models.TaskLog{}, &models.Host{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedTask(t *testing.T, db *gorm.DB, name string, status models.Status, tag string) {
	t.Helper()
	task := models.Task{
		Name:     name,
		Level:    models.TaskLevelParent,
		Spec:     "0 0 * * * *",
		Protocol: models.TaskHTTP,
		Command:  "https://example.com",
		Status:   status,
		Tag:      tag,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
}

func TestQueryTasks_FilterAndLimit(t *testing.T) {
	db := newTaskTestDB(t)
	seedTask(t, db, "backup-db", models.Enabled, "ops")
	seedTask(t, db, "cleanup-logs", models.Disabled, "ops")
	seedTask(t, db, "report", models.Enabled, "biz")

	// 全部
	all, err := queryTasks(db, taskListFilter{})
	if err != nil {
		t.Fatalf("queryTasks: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	// 倒序：最后插入的 report 在最前
	if all[0].Name != "report" {
		t.Fatalf("expected DESC order, first=%s", all[0].Name)
	}

	// 状态过滤
	enabled := models.Enabled
	en, _ := queryTasks(db, taskListFilter{Status: &enabled})
	if len(en) != 2 {
		t.Fatalf("expected 2 enabled, got %d", len(en))
	}

	// 名称过滤
	byName, _ := queryTasks(db, taskListFilter{Name: "clean"})
	if len(byName) != 1 || byName[0].Name != "cleanup-logs" {
		t.Fatalf("name filter failed: %+v", byName)
	}

	// 标签过滤
	byTag, _ := queryTasks(db, taskListFilter{Tag: "biz"})
	if len(byTag) != 1 || byTag[0].Name != "report" {
		t.Fatalf("tag filter failed: %+v", byTag)
	}

	// limit
	limited, _ := queryTasks(db, taskListFilter{Limit: 1})
	if len(limited) != 1 {
		t.Fatalf("limit failed: %d", len(limited))
	}
}

func TestQueryTaskLogs(t *testing.T) {
	db := newTaskTestDB(t)
	logs := []models.TaskLog{
		{Id: 1, TaskId: 1, Name: "a", Status: models.Finish},
		{Id: 2, TaskId: 1, Name: "a", Status: models.Failure},
		{Id: 3, TaskId: 2, Name: "b", Status: models.Finish},
	}
	for i := range logs {
		if err := db.Create(&logs[i]).Error; err != nil {
			t.Fatalf("seed log: %v", err)
		}
	}

	// 指定任务
	t1, err := queryTaskLogs(db, 1, 20)
	if err != nil {
		t.Fatalf("queryTaskLogs: %v", err)
	}
	if len(t1) != 2 {
		t.Fatalf("expected 2 logs for task 1, got %d", len(t1))
	}
	// 倒序
	if t1[0].Id != 2 {
		t.Fatalf("expected DESC by id, first id=%d", t1[0].Id)
	}

	// 全部
	all, _ := queryTaskLogs(db, 0, 20)
	if len(all) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(all))
	}
}

func TestGatherStats(t *testing.T) {
	db := newTaskTestDB(t)
	seedTask(t, db, "a", models.Enabled, "")
	seedTask(t, db, "b", models.Enabled, "")
	seedTask(t, db, "c", models.Disabled, "")
	if err := db.Create(&models.Host{Name: "node-1", Port: 5921}).Error; err != nil {
		t.Fatalf("seed host: %v", err)
	}
	if err := db.Create(&models.TaskLog{Id: 1, TaskId: 1, Name: "a", Status: models.Finish}).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}

	s, err := gatherStats(db)
	if err != nil {
		t.Fatalf("gatherStats: %v", err)
	}
	if s.TotalTasks != 3 || s.EnabledTasks != 2 || s.TotalHosts != 1 || s.TotalLogs != 1 {
		t.Fatalf("unexpected stats: %+v", s)
	}
}

func TestLabels(t *testing.T) {
	if taskStatusLabel(models.Enabled) != "enabled" || taskStatusLabel(models.Disabled) != "disabled" {
		t.Error("task status label wrong")
	}
	if protocolLabel(models.TaskHTTP) != "HTTP" || protocolLabel(models.TaskRPC) != "RPC" {
		t.Error("protocol label wrong")
	}
	cases := map[models.Status]string{
		models.Failure: "failed", models.Running: "running",
		models.Finish: "success", models.Cancel: "cancelled",
	}
	for s, want := range cases {
		if got := logStatusLabel(s); got != want {
			t.Errorf("logStatusLabel(%d)=%q want %q", s, got, want)
		}
	}
	if dashIfEmpty("") != "-" || dashIfEmpty(" ") != "-" || dashIfEmpty("x") != "x" {
		t.Error("dashIfEmpty wrong")
	}
	if formatLocalTime(models.LocalTime(time.Time{})) != "-" {
		t.Error("zero time should render as -")
	}
}

func TestFormatTables(t *testing.T) {
	// 空集
	if !strings.Contains(formatTasksTable(nil), "No tasks") {
		t.Error("empty tasks table wrong")
	}
	if !strings.Contains(formatLogsTable(nil), "No logs") {
		t.Error("empty logs table wrong")
	}
	// 含数据：表头 + 行
	out := formatTasksTable([]models.Task{
		{Id: 1, Name: "backup", Spec: "0 0 9 * * *", Protocol: models.TaskRPC, Status: models.Enabled, Tag: "ops"},
	})
	for _, want := range []string{"ID", "NAME", "backup", "RPC", "enabled", "ops"} {
		if !strings.Contains(out, want) {
			t.Errorf("tasks table missing %q in:\n%s", want, out)
		}
	}
}
