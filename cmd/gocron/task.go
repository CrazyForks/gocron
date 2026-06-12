package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/app"
	"github.com/gocronx-team/gocron/internal/modules/setting"
	"github.com/urfave/cli/v2"
	"gorm.io/gorm"
)

// bootstrapDB 引导配置与 DB（只读用途），不启动 web / 调度器 / 选举。
// 与 resetpwd 一致：把相对 SQLite 路径锚定到 gocron 基准目录，连到服务端同一个库。
func bootstrapDB() error {
	gin.SetMode(gin.ReleaseMode)
	app.InitEnv(AppVersion)
	if !app.Installed {
		return errors.New("gocron is not installed")
	}
	config, err := setting.Read(app.AppConfig)
	if err != nil {
		return fmt.Errorf("failed to read config (%s): %w", app.AppConfig, err)
	}
	config.Db.Database = resolveSQLitePath(config.Db.Engine, config.Db.Database, filepath.Dir(app.AppDir))
	app.Setting = config
	models.Db = models.CreateDb()
	return nil
}

// taskCommand 只读地查询任务与执行日志（不影响运行中的调度器）。
func taskCommand() *cli.Command {
	return &cli.Command{
		Name:  "task",
		Usage: "inspect tasks and execution logs (read-only)",
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "list tasks",
				Action: runTaskList,
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "status", Usage: "filter by status: enabled|disabled"},
					&cli.StringFlag{Name: "name", Usage: "filter by name (substring)"},
					&cli.StringFlag{Name: "tag", Usage: "filter by tag (substring)"},
					&cli.IntFlag{Name: "limit", Value: 20, Usage: "max rows"},
					&cli.BoolFlag{Name: "json", Usage: "output JSON"},
				},
			},
			{
				Name:      "logs",
				Usage:     "show recent execution logs (optionally for one task id)",
				ArgsUsage: "[taskId]",
				Action:    runTaskLogs,
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "limit", Value: 20, Usage: "max rows"},
					&cli.BoolFlag{Name: "json", Usage: "output JSON"},
				},
			},
		},
	}
}

// statsCommand 打印任务/主机/日志的概览计数。
func statsCommand() *cli.Command {
	return &cli.Command{
		Name:   "stats",
		Usage:  "show a summary (task / host / log counts)",
		Action: runStats,
		Flags:  []cli.Flag{&cli.BoolFlag{Name: "json", Usage: "output JSON"}},
	}
}

// region command actions

func runTaskList(ctx *cli.Context) error {
	if err := bootstrapDB(); err != nil {
		return err
	}
	f := taskListFilter{
		Name:  ctx.String("name"),
		Tag:   ctx.String("tag"),
		Limit: ctx.Int("limit"),
	}
	switch strings.ToLower(strings.TrimSpace(ctx.String("status"))) {
	case "enabled":
		s := models.Enabled
		f.Status = &s
	case "disabled":
		s := models.Disabled
		f.Status = &s
	case "":
		// no status filter
	default:
		return fmt.Errorf("invalid --status %q (use enabled|disabled)", ctx.String("status"))
	}

	tasks, err := queryTasks(models.Db, f)
	if err != nil {
		return fmt.Errorf("query tasks: %w", err)
	}
	if ctx.Bool("json") {
		return printJSON(tasks)
	}
	fmt.Print(formatTasksTable(tasks))
	return nil
}

func runTaskLogs(ctx *cli.Context) error {
	if err := bootstrapDB(); err != nil {
		return err
	}
	taskID := 0
	if ctx.Args().Len() >= 1 {
		id, err := strconv.Atoi(ctx.Args().Get(0))
		if err != nil || id <= 0 {
			return fmt.Errorf("invalid task id %q", ctx.Args().Get(0))
		}
		taskID = id
	}
	logs, err := queryTaskLogs(models.Db, taskID, ctx.Int("limit"))
	if err != nil {
		return fmt.Errorf("query logs: %w", err)
	}
	if ctx.Bool("json") {
		return printJSON(logs)
	}
	fmt.Print(formatLogsTable(logs))
	return nil
}

func runStats(ctx *cli.Context) error {
	if err := bootstrapDB(); err != nil {
		return err
	}
	s, err := gatherStats(models.Db)
	if err != nil {
		return fmt.Errorf("gather stats: %w", err)
	}
	if ctx.Bool("json") {
		return printJSON(s)
	}
	fmt.Printf("Tasks:     %d (enabled %d, disabled %d)\n", s.TotalTasks, s.EnabledTasks, s.TotalTasks-s.EnabledTasks)
	fmt.Printf("Hosts:     %d\n", s.TotalHosts)
	fmt.Printf("Exec logs: %d\n", s.TotalLogs)
	return nil
}

// endregion

// region pure logic (no I/O, unit-testable)

type taskListFilter struct {
	Status *models.Status // nil = all
	Name   string
	Tag    string
	Limit  int
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	return limit
}

func queryTasks(db *gorm.DB, f taskListFilter) ([]models.Task, error) {
	q := db.Model(&models.Task{})
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	if f.Name != "" {
		q = q.Where("name LIKE ?", "%"+f.Name+"%")
	}
	if f.Tag != "" {
		q = q.Where("tag LIKE ?", "%"+f.Tag+"%")
	}
	tasks := make([]models.Task, 0)
	err := q.Order("id DESC").Limit(normalizeLimit(f.Limit)).Find(&tasks).Error
	return tasks, err
}

func queryTaskLogs(db *gorm.DB, taskID, limit int) ([]models.TaskLog, error) {
	q := db.Model(&models.TaskLog{})
	if taskID > 0 {
		q = q.Where("task_id = ?", taskID)
	}
	logs := make([]models.TaskLog, 0)
	err := q.Order("id DESC").Limit(normalizeLimit(limit)).Find(&logs).Error
	return logs, err
}

type statsResult struct {
	TotalTasks   int64 `json:"total_tasks"`
	EnabledTasks int64 `json:"enabled_tasks"`
	TotalHosts   int64 `json:"total_hosts"`
	TotalLogs    int64 `json:"total_logs"`
}

func gatherStats(db *gorm.DB) (statsResult, error) {
	var s statsResult
	if err := db.Model(&models.Task{}).Count(&s.TotalTasks).Error; err != nil {
		return s, err
	}
	if err := db.Model(&models.Task{}).Where("status = ?", models.Enabled).Count(&s.EnabledTasks).Error; err != nil {
		return s, err
	}
	if err := db.Model(&models.Host{}).Count(&s.TotalHosts).Error; err != nil {
		return s, err
	}
	if err := db.Model(&models.TaskLog{}).Count(&s.TotalLogs).Error; err != nil {
		return s, err
	}
	return s, nil
}

// endregion

// region formatting

func taskStatusLabel(s models.Status) string {
	if s == models.Enabled {
		return "enabled"
	}
	return "disabled"
}

func protocolLabel(p models.TaskProtocol) string {
	switch p {
	case models.TaskHTTP:
		return "HTTP"
	case models.TaskRPC:
		return "RPC"
	default:
		return strconv.Itoa(int(p))
	}
}

func logStatusLabel(s models.Status) string {
	switch s {
	case models.Failure: // 0
		return "failed"
	case models.Running: // 1
		return "running"
	case models.Finish: // 2
		return "success"
	case models.Cancel: // 3
		return "cancelled"
	default:
		return strconv.Itoa(int(s))
	}
}

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func formatLocalTime(t models.LocalTime) string {
	gt := time.Time(t)
	if gt.IsZero() {
		return "-"
	}
	return gt.Format("2006-01-02 15:04:05")
}

func formatTasksTable(tasks []models.Task) string {
	if len(tasks) == 0 {
		return "No tasks found.\n"
	}
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSPEC\tPROTOCOL\tSTATUS\tTAG")
	for _, t := range tasks {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			t.Id, t.Name, t.Spec, protocolLabel(t.Protocol), taskStatusLabel(t.Status), dashIfEmpty(t.Tag))
	}
	_ = w.Flush()
	return b.String()
}

func formatLogsTable(logs []models.TaskLog) string {
	if len(logs) == 0 {
		return "No logs found.\n"
	}
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTASK\tNAME\tSTATUS\tSTART")
	for _, l := range logs {
		fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%s\n",
			l.Id, l.TaskId, l.Name, logStatusLabel(l.Status), formatLocalTime(l.StartTime))
	}
	_ = w.Flush()
	return b.String()
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// endregion
