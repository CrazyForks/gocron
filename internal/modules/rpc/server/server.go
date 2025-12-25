package server

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gocronx-team/gocron/internal/modules/rpc/auth"
	pb "github.com/gocronx-team/gocron/internal/modules/rpc/proto"
	"github.com/gocronx-team/gocron/internal/modules/utils"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type Server struct{}

var keepAlivePolicy = keepalive.EnforcementPolicy{
	MinTime:             10 * time.Second,
	PermitWithoutStream: true,
}

var keepAliveParams = keepalive.ServerParameters{
	MaxConnectionIdle: 30 * time.Second,
	Time:              30 * time.Second,
	Timeout:           3 * time.Second,
}

func (s Server) Run(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			log.Error(err)
		}
	}()

	// 清理 HTML 实体
	cleanedCmd := utils.CleanHTMLEntities(req.Command)
	log.Infof("execute cmd start: [id: %d]", req.Id)

	// 使用任务超时创建独立的 context，而不是直接使用客户端传来的 ctx
	timeout := time.Duration(req.Timeout) * time.Second
	taskCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	// 监听客户端取消，如果客户端取消了，也取消任务
	go func() {
		<-ctx.Done()
		cancel()
	}()

	// 直接调用 ExecShell，它会处理 context 取消并返回已有输出
	output, execErr := utils.ExecShell(taskCtx, cleanedCmd)
	
	log.Infof("[Output Length: %d bytes]", len(output))
	resp := new(pb.TaskResponse)
	resp.Output = output
	if execErr != nil {
		resp.Error = execErr.Error()
	} else {
		resp.Error = ""
	}
	log.Infof("execute cmd end: [id: %d err: %s]", req.Id, resp.Error)

	return resp, nil
}

func Start(addr string, enableTLS bool, certificate auth.Certificate) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepAliveParams),
		grpc.KeepaliveEnforcementPolicy(keepAlivePolicy),
	}
	if enableTLS {
		tlsConfig, err := certificate.GetTLSConfigForServer()
		if err != nil {
			log.Fatal(err)
		}
		opt := grpc.Creds(credentials.NewTLS(tlsConfig))
		opts = append(opts, opt)
	}
	server := grpc.NewServer(opts...)
	pb.RegisterTaskServer(server, Server{})
	log.Infof("server listen on %s", addr)

	go func() {
		err = server.Serve(l)
		if err != nil {
			log.Fatal(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	for {
		s := <-c
		log.Infoln("收到信号 -- ", s)
		switch s {
		case syscall.SIGHUP:
			log.Infoln("收到终端断开信号, 忽略")
		case syscall.SIGINT, syscall.SIGTERM:
			log.Info("应用准备退出")
			server.GracefulStop()
			return
		}
	}

}
