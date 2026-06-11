package mcp

import (
	"context"
	"errors"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "gocron-mcp"
	serverVersion = "1.0.0"
)

// errAdminRequired 在非管理员调用受限工具时返回。
var errAdminRequired = errors.New("permission denied: this operation requires an admin token")

// Handler 返回挂载在 /mcp 的 Streamable HTTP 处理器。
// 每个请求按 Auth 中间件写入 context 的用户身份构建一个绑定该用户权限的 MCP server，
// 工具调用因此天然继承令牌归属用户的权限（普通用户 / 管理员）。
func Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		u, ok := userFromContext(r.Context())
		if !ok {
			// Auth 中间件保证身份存在；缺失时返回 nil，SDK 会回 400。
			return nil
		}
		return newServerForUser(u)
	}, nil)
}

func newServerForUser(u *authUser) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)
	registerTools(s, u)
	return s
}

func registerTools(s *mcp.Server, u *authUser) {
	// 说明：返回 GORM models 的工具（list_tasks / get_task / query_task_logs / list_hosts）其 Out 用 any。
	// 原因是这些模型的 JSON 形态与反射推断出的 Schema 不一致，会导致 SDK 输出校验失败：
	//   - 自定义序列化时间类型（NextRunTime、LocalTime）序列化为字符串，但 Schema 推断为 object；
	//   - 内嵌的 BaseModel 含导出字段 Page/PageSize，被 Schema 标记为 required，但 JSON 用 `json:"-"` 忽略。
	// 用 any 时 SDK 跳过输出 Schema 校验，数据仍以 structuredContent 完整返回；输入 Schema 不受影响。
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_tasks",
		Description: "列出定时任务，支持按名称、标签、状态过滤与分页。",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listTasksInput) (*mcp.CallToolResult, any, error) {
		out, err := listTasks(in)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_task",
		Description: "按 ID 获取单个定时任务的详细配置。",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getTaskInput) (*mcp.CallToolResult, any, error) {
		out, err := getTask(in)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "query_task_logs",
		Description: "查询任务执行日志，支持按任务 ID、执行状态过滤与分页。",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in queryTaskLogsInput) (*mcp.CallToolResult, any, error) {
		out, err := queryTaskLogs(in)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_hosts",
		Description: "列出全部执行节点（主机）。",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		out, err := listHosts()
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "run_task",
		Description: "立即手动触发执行一个任务（需要管理员令牌）。",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in runTaskInput) (*mcp.CallToolResult, runTaskOutput, error) {
		if !u.IsAdmin {
			return nil, runTaskOutput{}, errAdminRequired
		}
		out, err := runTask(in)
		return nil, out, err
	})
}
