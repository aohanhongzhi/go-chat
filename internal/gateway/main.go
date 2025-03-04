package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/urfave/cli/v2"
	"go-chat/config"
	"go-chat/internal/pkg/im"
	"go-chat/internal/pkg/logger"
	"golang.org/x/sync/errgroup"
)

func main() {
	cmd := cli.NewApp()

	cmd.Name = "LumenIM 在线聊天"
	cmd.Usage = "IM Server"

	// 设置参数
	cmd.Flags = []cli.Flag{
		// 配置文件参数
		&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "./config.yaml", Usage: "配置文件路径", DefaultText: "./config.yaml"},
	}

	cmd.Action = newApp

	_ = cmd.Run(os.Args)
}

func newApp(tx *cli.Context) error {
	eg, groupCtx := errgroup.WithContext(tx.Context)

	// 初始化 IM 渠道配置
	im.Initialize(groupCtx, eg)

	// 读取配置文件
	conf := config.ReadConfig(tx.String("config"))

	// 设置日志输出
	logger.SetOutput(conf.GetLogPath(), "logger-ws")

	if !conf.Debug() {
		gin.SetMode(gin.ReleaseMode)
	}

	app := Initialize(conf)

	c := make(chan os.Signal, 1)

	signal.Notify(c, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	// 延时启动守护协程
	time.AfterFunc(3*time.Second, func() {
		app.Coroutine.Start(eg, groupCtx)
	})

	log.Printf("Server ID   :%s", conf.ServerId())
	log.Printf("Server Pid  :%d", os.Getpid())
	log.Printf("Websocket Listen Port :%d", conf.Ports.Websocket)
	log.Printf("Tcp Listen Port :%d", conf.Ports.Tcp)

	go NewTcpServer(app)

	return start(c, eg, groupCtx, app.Server)
}

func start(c chan os.Signal, eg *errgroup.Group, ctx context.Context, server *http.Server) error {

	eg.Go(func() error {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Websocket Listen Err: %s", err)
		}

		return err
	})

	eg.Go(func() error {
		defer func() {
			// 等待中断信号以优雅地关闭服务器（设置 5 秒的超时时间）
			timeCtx, timeCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer timeCancel()

			if err := server.Shutdown(timeCtx); err != nil {
				log.Printf("Websocket Shutdown Err: %s \n", err)
			}
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c:
			return nil
		}
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		time.Sleep(3 * time.Second)
	}

	log.Fatal("IM Server Shutdown")

	return nil
}
