package statistics

import (
	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/logger"
	"github.com/gocronx-team/gocron/internal/modules/utils"
	"github.com/gocronx-team/gocron/internal/routers/base"
)

// OverviewData 概览统计数据
type OverviewData struct {
	TotalTasks      int64               `json:"total_tasks"`
	TodayExecutions int64               `json:"today_executions"`
	SuccessRate     float64             `json:"success_rate"`
	FailedCount     int64               `json:"failed_count"`
	Last7Days       []models.DailyStats `json:"last_7_days"`
}

// Overview 获取统计概览数据
func Overview(c *gin.Context) {
	logger.Info("Starting to fetch statistics data")

	taskModel := models.Task{}
	taskLogModel := models.TaskLog{}

	// 1. 获取启用的任务总数
	logger.Info("Step 1: Getting total tasks count")
	totalTasks, err := taskModel.Total(models.CommonMap{"Status": int(models.Enabled)})
	if err != nil {
		logger.Error("Failed to get total tasks:", err)
		base.RespondError(c, "Failed to get total tasks", err)
		return
	}
	logger.Info("Total tasks:", totalTasks)

	// 2. 获取今日统计数据
	logger.Info("Step 2: Getting today's statistics")
	todayTotal, todaySuccess, todayFailed, err := taskLogModel.GetTodayStats()
	if err != nil {
		logger.Error("Failed to get today's statistics:", err)
		base.RespondError(c, "Failed to get today's statistics", err)
		return
	}
	logger.Info("Today's stats - Total:", todayTotal, "Success:", todaySuccess, "Failed:", todayFailed)

	// 3. 计算成功率
	var successRate float64
	if todayTotal > 0 {
		successRate = float64(todaySuccess) / float64(todayTotal) * 100
		// 保留1位小数
		successRate = float64(int(successRate*10)) / 10
	}
	logger.Info("Success rate:", successRate)

	// 4. 获取最近7天趋势
	logger.Info("Step 3: Getting last 7 days trend")
	last7Days, err := taskLogModel.GetLast7DaysTrend()
	if err != nil {
		logger.Error("Failed to get trend data:", err)
		base.RespondError(c, "Failed to get trend data", err)
		return
	}
	logger.Info("Last 7 days trend data count:", len(last7Days))

	// 组装返回数据
	data := OverviewData{
		TotalTasks:      totalTasks,
		TodayExecutions: todayTotal,
		SuccessRate:     successRate,
		FailedCount:     todayFailed,
		Last7Days:       last7Days,
	}

	logger.Info("Preparing to return data")
	base.RespondSuccess(c, utils.SuccessContent, data)
	logger.Info("Statistics data returned successfully")
}
