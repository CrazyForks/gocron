package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

// TestCleanupIntegration 端到端验证任务级日志清理
func TestCleanupIntegration(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	oldDb := Db
	Db = db
	defer func() { Db = oldDb }()

	Db.AutoMigrate(&Task{}, &TaskLog{})

	// 创建任务: task 10 保留2天, task 20 保留7天, task 30 无自定义(0)
	Db.Exec("INSERT INTO tasks (id, name, log_retention_days, level, protocol, spec, command, status, tag) VALUES (10, 'task-2day', 2, 1, 2, '@daily', 'ls', 0, '')")
	Db.Exec("INSERT INTO tasks (id, name, log_retention_days, level, protocol, spec, command, status, tag) VALUES (20, 'task-7day', 7, 1, 2, '@daily', 'ls', 0, '')")
	Db.Exec("INSERT INTO tasks (id, name, log_retention_days, level, protocol, spec, command, status, tag) VALUES (30, 'task-global', 0, 1, 2, '@daily', 'ls', 0, '')")

	now := time.Now()

	// 插入日志
	//  Task 10: 5条1天前(应保留), 5条3天前(应删除), 5条10天前(应删除)
	//  Task 20: 5条3天前(应保留), 5条10天前(应删除)
	//  Task 30: 5条10天前(无自定义策略，不由任务级清理处理)
	insertLogs := func(taskId int, name string, age time.Duration, count int) {
		for i := 0; i < count; i++ {
			Db.Create(&TaskLog{
				TaskId:    taskId,
				Name:      name,
				Spec:      "@daily",
				Protocol:  2,
				Command:   "ls",
				StartTime: LocalTime(now.Add(-age)),
				EndTime:   LocalTime(now.Add(-age).Add(time.Second)),
				Status:    Finish,
				Result:    "ok",
			})
		}
	}

	insertLogs(10, "task-2day", 1*24*time.Hour, 5)    // 1天前 → 保留
	insertLogs(10, "task-2day", 3*24*time.Hour, 5)    // 3天前 → 删除
	insertLogs(10, "task-2day", 10*24*time.Hour, 5)   // 10天前 → 删除
	insertLogs(20, "task-7day", 3*24*time.Hour, 5)    // 3天前 → 保留
	insertLogs(20, "task-7day", 10*24*time.Hour, 5)   // 10天前 → 删除
	insertLogs(30, "task-global", 10*24*time.Hour, 5) // 不由任务级策略处理

	// 验证初始状态
	var count10, count20, count30 int64
	Db.Model(&TaskLog{}).Where("task_id = 10").Count(&count10)
	Db.Model(&TaskLog{}).Where("task_id = 20").Count(&count20)
	Db.Model(&TaskLog{}).Where("task_id = 30").Count(&count30)
	fmt.Printf("Before cleanup - Task10: %d, Task20: %d, Task30: %d\n", count10, count20, count30)

	if count10 != 15 || count20 != 10 || count30 != 5 {
		t.Fatalf("Initial state wrong: %d, %d, %d", count10, count20, count30)
	}

	// 模拟 cron 清理逻辑: 查找自定义保留天数的任务并清理
	taskLogModel := new(TaskLog)
	var tasks []Task
	Db.Where("log_retention_days > 0").Find(&tasks)

	if len(tasks) != 2 {
		t.Fatalf("Expected 2 tasks with custom retention, got %d", len(tasks))
	}

	for _, task := range tasks {
		count, err := taskLogModel.RemoveByTaskIdAndDays(task.Id, task.LogRetentionDays)
		if err != nil {
			t.Fatalf("RemoveByTaskIdAndDays failed for task %d: %v", task.Id, err)
		}
		fmt.Printf("Task %d (%s, retention=%d days): deleted %d logs\n",
			task.Id, task.Name, task.LogRetentionDays, count)
	}

	// 验证清理后状态
	Db.Model(&TaskLog{}).Where("task_id = 10").Count(&count10)
	Db.Model(&TaskLog{}).Where("task_id = 20").Count(&count20)
	Db.Model(&TaskLog{}).Where("task_id = 30").Count(&count30)
	fmt.Printf("After cleanup - Task10: %d, Task20: %d, Task30: %d\n", count10, count20, count30)

	// Task 10 (保留2天): 应只剩1天前的5条
	if count10 != 5 {
		t.Errorf("Task 10: expected 5 logs remaining (1-day-old), got %d", count10)
	}
	// Task 20 (保留7天): 应只剩3天前的5条
	if count20 != 5 {
		t.Errorf("Task 20: expected 5 logs remaining (3-day-old), got %d", count20)
	}
	// Task 30 (无自定义): 应该不受影响，仍有5条
	if count30 != 5 {
		t.Errorf("Task 30: expected 5 logs untouched, got %d", count30)
	}

	fmt.Println("\n✅ 任务级日志清理验证通过!")
	fmt.Println("  - Task 10 (2天保留): 删除了3天前和10天前的日志，保留了1天前的")
	fmt.Println("  - Task 20 (7天保留): 删除了10天前的日志，保留了3天前的")
	fmt.Println("  - Task 30 (全局策略): 不受任务级清理影响")
}
