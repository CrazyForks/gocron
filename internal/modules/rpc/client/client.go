package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/status"

	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/logger"
	"github.com/gocronx-team/gocron/internal/modules/rpc/grpcpool"
	pb "github.com/gocronx-team/gocron/internal/modules/rpc/proto"
	"google.golang.org/grpc/codes"
)

var (
	taskCtxMap     sync.Map // 存储任务执行的 context.CancelFunc
	errUnavailable = errors.New("无法连接远程服务器")
)

func generateTaskUniqueKey(ip string, port int, id int64) string {
	return fmt.Sprintf("%s:%d:%d", ip, port, id)
}

func Stop(ip string, port int, id int64) {
	key := generateTaskUniqueKey(ip, port, id)
	logger.Infof("尝试停止任务#key-%s#taskLogId-%d", key, id)
	cancel, ok := taskCtxMap.Load(key)
	if !ok {
		logger.Warnf("未找到运行中的任务，可能是历史任务，直接更新数据库状态#key-%s", key)
		updateOrphanedTaskLog(id)
		return
	}
	logger.Infof("找到运行中的任务，取消context#key-%s", key)
	cancel.(context.CancelFunc)()
}

func Exec(ip string, port int, taskReq *pb.TaskRequest) (string, error) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error("panic#rpc/client.go:Exec#", err)
		}
	}()
	addr := fmt.Sprintf("%s:%d", ip, port)
	c, err := grpcpool.Pool.Get(addr)
	if err != nil {
		return "", err
	}
	if taskReq.Timeout <= 0 || taskReq.Timeout > 86400 {
		taskReq.Timeout = 86400
	}
	timeout := time.Duration(taskReq.Timeout) * time.Second
	// RPC context: 比任务超时多5秒，给服务端时间清理进程并返回输出
	ctx, cancel := context.WithTimeout(context.Background(), timeout+5*time.Second)
	defer cancel()

	taskUniqueKey := generateTaskUniqueKey(ip, port, taskReq.Id)
	logger.Infof("任务开始执行，存储cancel函数#key-%s#taskLogId-%d", taskUniqueKey, taskReq.Id)
	taskCtxMap.Store(taskUniqueKey, cancel)
	defer func() {
		logger.Infof("任务执行完成，删除cancel函数#key-%s", taskUniqueKey)
		taskCtxMap.Delete(taskUniqueKey)
	}()

	resp, err := c.Run(ctx, taskReq)

	// 处理响应：即使有错误，也要返回已产生的输出
	if err != nil {
		if resp != nil && resp.Output != "" {
			return resp.Output, parseGRPCErrorOnly(err)
		}
		return parseGRPCError(err)
	}

	if resp.Error == "" {
		return resp.Output, nil
	}

	return resp.Output, errors.New(resp.Error)
}

func parseGRPCError(err error) (string, error) {
	switch status.Code(err) {
	case codes.Unavailable:
		return "", errUnavailable
	case codes.DeadlineExceeded:
		return "", errors.New("执行超时, 强制结束")
	case codes.Canceled:
		return "", errors.New("手动停止")
	}
	return "", err
}

// parseGRPCErrorOnly 只返回错误，不返回输出
func parseGRPCErrorOnly(err error) error {
	switch status.Code(err) {
	case codes.Unavailable:
		return errUnavailable
	case codes.DeadlineExceeded:
		return errors.New("执行超时, 强制结束")
	case codes.Canceled:
		return errors.New("手动停止")
	}
	return err
}

// 处理孤立的任务日志（重启后丢失的任务）
func updateOrphanedTaskLog(taskLogId int64) {
	taskLogModel := new(models.TaskLog)
	_, err := taskLogModel.Update(taskLogId, models.CommonMap{
		"status": models.Cancel,
		"result": "系统重启后手动停止",
	})
	if err != nil {
		logger.Errorf("更新孤立任务日志状态失败#taskLogId-%d#错误-%s", taskLogId, err.Error())
	} else {
		logger.Infof("已更新孤立任务日志状态为已取消#taskLogId-%d", taskLogId)
	}
}
