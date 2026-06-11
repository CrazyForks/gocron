package mcp

import (
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/service"
)

const maxPageSize = 100

// ── list_tasks ──────────────────────────────────────────────────────────────

type listTasksInput struct {
	Name     string `json:"name,omitempty" jsonschema:"按任务名称模糊过滤"`
	Tag      string `json:"tag,omitempty" jsonschema:"按标签模糊过滤"`
	Status   *int   `json:"status,omitempty" jsonschema:"按状态过滤，0 表示禁用，1 表示启用，省略则返回全部"`
	Page     int    `json:"page,omitempty" jsonschema:"页码，从 1 开始，默认 1"`
	PageSize int    `json:"page_size,omitempty" jsonschema:"每页数量，默认 20，最大 100"`
}

type listTasksOutput struct {
	Total int64         `json:"total"`
	Tasks []models.Task `json:"tasks"`
}

func listTasks(in listTasksInput) (listTasksOutput, error) {
	params := models.CommonMap{
		"Page":     normalizePage(in.Page),
		"PageSize": normalizePageSize(in.PageSize),
	}
	if in.Name != "" {
		params["Name"] = in.Name
	}
	if in.Tag != "" {
		params["Tag"] = in.Tag
	}
	if in.Status != nil {
		params["Status"] = *in.Status
	}

	taskModel := new(models.Task)
	total, err := taskModel.Total(params)
	if err != nil {
		return listTasksOutput{}, err
	}
	tasks, err := taskModel.List(params)
	if err != nil {
		return listTasksOutput{}, err
	}
	for i, item := range tasks {
		tasks[i].NextRunTime = models.NextRunTime(service.ServiceTask.NextRunTime(item))
	}
	return listTasksOutput{Total: total, Tasks: tasks}, nil
}

// ── get_task ────────────────────────────────────────────────────────────────

type getTaskInput struct {
	Id int `json:"id" jsonschema:"任务 ID"`
}

func getTask(in getTaskInput) (models.Task, error) {
	taskModel := new(models.Task)
	task, err := taskModel.Detail(in.Id)
	if err != nil {
		return models.Task{}, err
	}
	if task.Id != 0 {
		task.NextRunTime = models.NextRunTime(service.ServiceTask.NextRunTime(task))
	}
	return task, nil
}

// ── query_task_logs ───────────────────────────────────────────────────────────

type queryTaskLogsInput struct {
	TaskId   int  `json:"task_id,omitempty" jsonschema:"按任务 ID 过滤，省略则返回全部任务的日志"`
	Status   *int `json:"status,omitempty" jsonschema:"按执行状态过滤，0 表示失败，1 表示成功，2 表示执行中"`
	Page     int  `json:"page,omitempty" jsonschema:"页码，从 1 开始，默认 1"`
	PageSize int  `json:"page_size,omitempty" jsonschema:"每页数量，默认 20，最大 100"`
}

type queryTaskLogsOutput struct {
	Total int64            `json:"total"`
	Logs  []models.TaskLog `json:"logs"`
}

func queryTaskLogs(in queryTaskLogsInput) (queryTaskLogsOutput, error) {
	params := models.CommonMap{
		"Page":     normalizePage(in.Page),
		"PageSize": normalizePageSize(in.PageSize),
	}
	if in.TaskId > 0 {
		params["TaskId"] = in.TaskId
	}
	if in.Status != nil {
		params["Status"] = *in.Status
	}

	logModel := new(models.TaskLog)
	total, err := logModel.Total(params)
	if err != nil {
		return queryTaskLogsOutput{}, err
	}
	logs, err := logModel.List(params)
	if err != nil {
		return queryTaskLogsOutput{}, err
	}
	return queryTaskLogsOutput{Total: total, Logs: logs}, nil
}

// ── list_hosts ────────────────────────────────────────────────────────────────

type listHostsOutput struct {
	Hosts []models.Host `json:"hosts"`
}

func listHosts() (listHostsOutput, error) {
	hostModel := new(models.Host)
	hosts, err := hostModel.List(models.CommonMap{"Page": 1, "PageSize": maxPageSize})
	if err != nil {
		return listHostsOutput{}, err
	}
	return listHostsOutput{Hosts: hosts}, nil
}

// ── run_task ──────────────────────────────────────────────────────────────────

type runTaskInput struct {
	Id int `json:"id" jsonschema:"要立即执行的任务 ID"`
}

type runTaskOutput struct {
	Started bool   `json:"started"`
	Message string `json:"message"`
}

func runTask(in runTaskInput) (runTaskOutput, error) {
	taskModel := new(models.Task)
	task, err := taskModel.Detail(in.Id)
	if err != nil {
		return runTaskOutput{}, err
	}
	if task.Id <= 0 {
		return runTaskOutput{Started: false, Message: "task not found"}, nil
	}
	task.Spec = "MCP manual run"
	service.ServiceTask.Run(task)
	return runTaskOutput{Started: true, Message: "task started, check logs for result"}, nil
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizePageSize(size int) int {
	if size <= 0 {
		return 20
	}
	if size > maxPageSize {
		return maxPageSize
	}
	return size
}
