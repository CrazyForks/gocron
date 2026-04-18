package task

import (
	"github.com/gin-gonic/gin"

	"github.com/gocronx-team/gocron/internal/modules/utils"
	"github.com/gocronx-team/gocron/internal/routers/base"
	"github.com/gocronx-team/gocron/internal/service"
)

type cronPreviewRequest struct {
	Spec     string `json:"spec" binding:"required"`
	Timezone string `json:"timezone"`
	Count    int    `json:"count"`
}

// CronPreview 返回给定 cron 表达式的接下来 N 次执行时间 + 一周执行分布热图。
// 非法表达式也返回 HTTP 200，body 里 valid=false（用户边敲边预览，不用 4xx 轰炸 console）。
func CronPreview(c *gin.Context) {
	var req cronPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		base.RespondValidationError(c, err)
		return
	}

	result := service.PreviewCron(req.Spec, req.Timezone, req.Count)

	jsonResp := utils.JsonResponse{}
	c.String(200, jsonResp.Success(utils.SuccessContent, result))
}
