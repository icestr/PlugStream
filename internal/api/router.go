package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/icestr/PlugStream/internal/config"
	"github.com/icestr/PlugStream/internal/logger"
	"github.com/icestr/PlugStream/internal/segment"
)

// SetupRouter 设置路由
func SetupRouter(cfg *config.Config, segMgr segment.Manager, version string) *gin.Engine {
	// 设置 Gin 模式
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由引擎
	router := gin.New()

	// 添加中间件
	router.Use(ginLogger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// 创建处理器
	handler := NewHandler(cfg, segMgr, version)

	// API 路由组
	api := router.Group("/api/v1")
	{
		// 健康检查
		api.GET("/health", handler.HealthCheck)

		// 视频相关接口
		video := api.Group("/video")
		{
			// 获取视频流
			video.GET("/stream", handler.StreamVideo)
			// 获取视频信息
			video.GET("/info", handler.GetVideoInfo)
			// 获取指定分片
			video.GET("/segment", handler.GetSegment)
			// HLS 相关接口
			video.GET("/hls/playlist", handler.GetHLSPlaylist)
			video.GET("/hls/segment", handler.GetHLSSegment)
		}
	}

	return router
}

// ginLogger Gin 日志中间件
func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录日志
		logger.WithFields(map[string]interface{}{
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"status":     c.Writer.Status(),
			"latency_ms": time.Since(start).Milliseconds(),
			"client_ip":  c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}).Info("HTTP 请求")
	}
}

// corsMiddleware CORS 中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Range")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
