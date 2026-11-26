package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/icestr/PlugStream/internal/api"
	"github.com/icestr/PlugStream/internal/config"
	"github.com/icestr/PlugStream/internal/logger"
	"github.com/icestr/PlugStream/internal/segment"
	"github.com/icestr/PlugStream/internal/transcoder"
)

var (
	configPath = flag.String("config", "config.yaml", "配置文件路径")
	version    = "0.1.0"
)

func main() {
	flag.Parse()

	// 打印版本信息
	fmt.Printf("PlugStream v%s\n", version)
	fmt.Println("一个基于 Go 和 FFmpeg 的实时视频转码流式传输服务")
	fmt.Println()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	if err := logger.Init(cfg); err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	logger.Info("PlugStream 启动中...")
	logger.Infof("配置文件: %s", *configPath)

	// 创建转码器
	trans, err := transcoder.NewFFmpegTranscoder(cfg)
	if err != nil {
		logger.Fatalf("创建转码器失败: %v", err)
	}
	defer trans.Close()
	logger.Info("转码器初始化成功")

	// 创建分片管理器
	segMgr, err := segment.NewSegmentManager(cfg, trans)
	if err != nil {
		logger.Fatalf("创建分片管理器失败: %v", err)
	}
	defer segMgr.Close()
	logger.Info("分片管理器初始化成功")

	// 设置路由
	router := api.SetupRouter(cfg, segMgr, version)

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:           cfg.GetServerAddress(),
		Handler:        router,
		ReadTimeout:    cfg.GetServerTimeout(),
		WriteTimeout:   cfg.GetServerTimeout(),
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// 启动服务器
	go func() {
		logger.Infof("HTTP 服务器启动在 %s", cfg.GetServerAddress())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("HTTP 服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("服务器关闭失败: %v", err)
	}

	logger.Info("服务器已关闭")
}
