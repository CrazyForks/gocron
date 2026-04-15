package audit

import (
	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/utils"
	"github.com/gocronx-team/gocron/internal/routers/base"
)

// Index lists audit logs with pagination and optional filters.
// Query params: page, page_size, module, action, username, start_date, end_date
func Index(c *gin.Context) {
	auditLogModel := new(models.AuditLog)
	params := models.CommonMap{}
	base.ParsePageAndPageSize(c, params)

	if module := c.Query("module"); module != "" {
		params["Module"] = module
	}
	if action := c.Query("action"); action != "" {
		params["Action"] = action
	}
	if username := c.Query("username"); username != "" {
		params["Username"] = username
	}
	if startDate := c.Query("start_date"); startDate != "" {
		params["StartDate"] = startDate
	}
	if endDate := c.Query("end_date"); endDate != "" {
		params["EndDate"] = endDate
	}

	total, err := auditLogModel.Total(params)
	if err != nil {
		base.RespondErrorWithDefaultMsg(c, err)
		return
	}
	list, err := auditLogModel.List(params)
	if err != nil {
		base.RespondErrorWithDefaultMsg(c, err)
		return
	}

	base.RespondSuccess(c, utils.SuccessContent, map[string]interface{}{
		"total": total,
		"data":  list,
	})
}
