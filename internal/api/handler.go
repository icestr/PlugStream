package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/icestr/PlugStream/internal/config"
	"github.com/icestr/PlugStream/internal/logger"
	"github.com/icestr/PlugStream/internal/segment"
)

// Handler API 处理器
type Handler struct {
	config         *config.Config
	segmentManager segment.Manager
	version        string
}

// NewHandler 创建 API 处理器
func NewHandler(cfg *config.Config, segMgr segment.Manager, version string) *Handler {
	if version == "" {
		version = "dev"
	}
	return &Handler{
		config:         cfg,
		segmentManager: segMgr,
		version:        version,
	}
}

// validateVideoURL 校验视频源地址，仅允许 http/https 且必须包含主机名
func validateVideoURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("无效的视频 URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("仅支持 http 或 https 协议的视频 URL")
	}
	if u.Host == "" {
		return fmt.Errorf("视频 URL 缺少主机名")
	}
	return nil
}

// StreamVideoRequest 视频流请求参数
type StreamVideoRequest struct {
	URL string `form:"url" binding:"required"` // 视频 URL
}

// StreamVideo 处理视频流请求
func (h *Handler) StreamVideo(c *gin.Context) {
	var req StreamVideoRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "缺少必需参数: url",
		})
		return
	}

	if err := validateVideoURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"video_url":    req.URL,
		"client_ip":    c.ClientIP(),
		"user_agent":   c.Request.UserAgent(),
		"range_header": c.GetHeader("Range"),
	}).Info("收到视频流请求")

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.config.GetServerTimeout())
	defer cancel()

	// 解析 Range 请求头
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		// 处理 Range 请求
		h.handleRangeRequest(c, ctx, req.URL, rangeHeader)
	} else {
		// 处理完整视频请求
		h.handleFullRequest(c, ctx, req.URL)
	}
}

// handleFullRequest 处理完整视频请求
func (h *Handler) handleFullRequest(c *gin.Context, ctx context.Context, videoURL string) {
	// 从第一个分片开始流式传输
	segmentIndex := 0

	// 设置响应头
	c.Header("Content-Type", "video/mp4")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "no-cache")

	// 获取并流式传输分片
	reader, err := h.segmentManager.GetSegment(ctx, videoURL, segmentIndex)
	if err != nil {
		logger.Errorf("获取视频分片失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "transcode_error",
			Message: "视频转码失败",
		})
		return
	}
	defer reader.Close()

	// 流式传输
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, reader); err != nil {
		logger.Errorf("流式传输失败: %v", err)
	}
}

// handleRangeRequest 处理 Range 请求
// 实现符合 RFC 7233 标准的 HTTP Range 请求处理
// 支持三种格式：
// 1. bytes=start-end  (指定范围)
// 2. bytes=start-     (从start到末尾)
// 3. bytes=-suffix    (最后suffix字节)
func (h *Handler) handleRangeRequest(c *gin.Context, ctx context.Context, videoURL, rangeHeader string) {
	// 解析 Range 头
	rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeHeader, "-")

	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_range",
			Message: "无效的 Range 请求头格式",
		})
		return
	}

	var start, end int64 = -1, -1
	var isSuffixRange bool

	// 检查是否是后缀范围请求 (bytes=-500)
	if parts[0] == "" && parts[1] != "" {
		// 后缀范围：请求最后 N 字节
		suffixLength, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil || suffixLength <= 0 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_range",
				Message: "无效的后缀长度",
			})
			return
		}
		isSuffixRange = true
		end = suffixLength // 暂存后缀长度，稍后根据文件大小计算实际起始位置
	} else if parts[0] != "" {
		// 普通范围请求
		var err error
		start, err = strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil || start < 0 {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_range",
				Message: "无效的起始位置",
			})
			return
		}

		// 解析结束位置（可选）
		if parts[1] != "" {
			end, err = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			if err != nil || end < start {
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error:   "invalid_range",
					Message: "无效的结束位置",
				})
				return
			}
		} else {
			end = -1 // 表示到文件末尾
		}
	} else {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_range",
			Message: "无效的 Range 请求头",
		})
		return
	}

	// 根据字节位置计算对应的分片索引
	segmentDuration := h.config.Segment.Duration
	estimatedBytesPerSecond := int64(2 * 1024 * 1024 / 8) // 假设 2Mbps
	estimatedBytesPerSegment := estimatedBytesPerSecond * int64(segmentDuration)

	var segmentIndex int
	if !isSuffixRange {
		segmentIndex = int(start / estimatedBytesPerSegment)
	} else {
		// 后缀范围请求，需要先获取总大小
		// 暂时使用第一个分片，稍后根据文件大小调整
		segmentIndex = 0
	}

	logger.WithFields(map[string]interface{}{
		"video_url":     videoURL,
		"range_start":   start,
		"range_end":     end,
		"is_suffix":     isSuffixRange,
		"segment_index": segmentIndex,
	}).Info("处理 Range 请求")

	// 获取分片文件路径
	segmentPath := h.segmentManager.GetSegmentPath(videoURL, segmentIndex)

	// 检查分片是否存在，如果不存在则转码
	var fileSize int64
	if stat, err := os.Stat(segmentPath); err == nil {
		fileSize = stat.Size()
	} else {
		// 分片不存在，需要转码
		reader, err := h.segmentManager.GetSegment(ctx, videoURL, segmentIndex)
		if err != nil {
			logger.Errorf("获取视频分片失败: %v", err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "transcode_error",
				Message: "视频转码失败",
			})
			return
		}
		defer reader.Close()

		// 对于新转码的分片，无法准确知道大小，返回整个分片
		c.Header("Content-Type", "video/mp4")
		c.Header("Accept-Ranges", "bytes")
		c.Header("Cache-Control", "no-cache")
		c.Status(http.StatusPartialContent)

		if _, err := io.Copy(c.Writer, reader); err != nil {
			logger.Errorf("流式传输失败: %v", err)
		}
		return
	}

	// 打开已缓存的分片文件
	file, err := os.Open(segmentPath)
	if err != nil {
		logger.Errorf("打开分片文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "file_error",
			Message: "打开文件失败",
		})
		return
	}
	defer file.Close()

	// 计算实际的字节范围
	var segmentOffset, contentLength int64

	if isSuffixRange {
		// 后缀范围：请求最后 N 字节
		suffixLength := end // 之前暂存的后缀长度
		if suffixLength > fileSize {
			suffixLength = fileSize
		}
		segmentOffset = fileSize - suffixLength
		contentLength = suffixLength
		end = fileSize - 1
		start = segmentOffset
	} else {
		// 普通范围请求
		segmentOffset = start % estimatedBytesPerSegment
		if segmentOffset >= fileSize {
			segmentOffset = 0
		}

		// 如果没有指定结束位置，读取到文件末尾
		if end == -1 || end >= fileSize {
			end = fileSize - 1
		}

		// 计算内容长度
		contentLength = end - segmentOffset + 1
		if contentLength > fileSize-segmentOffset {
			contentLength = fileSize - segmentOffset
		}
	}

	// 设置响应头
	c.Header("Content-Type", "video/mp4")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", segmentOffset, segmentOffset+contentLength-1, fileSize))
	c.Header("Cache-Control", "public, max-age=3600")

	// 定位到起始位置
	if _, err := file.Seek(segmentOffset, io.SeekStart); err != nil {
		logger.Errorf("文件定位失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "seek_error",
			Message: "文件定位失败",
		})
		return
	}

	// 返回部分内容
	c.Status(http.StatusPartialContent)

	// 使用 LimitReader 限制读取长度
	limitedReader := io.LimitReader(file, contentLength)
	if _, err := io.Copy(c.Writer, limitedReader); err != nil {
		logger.Errorf("流式传输失败: %v", err)
	}

	logger.WithFields(map[string]interface{}{
		"segment_index":  segmentIndex,
		"segment_offset": segmentOffset,
		"content_length": contentLength,
		"file_size":      fileSize,
	}).Debug("Range 请求处理完成")
}

// GetVideoInfo 获取视频信息
func (h *Handler) GetVideoInfo(c *gin.Context) {
	var req StreamVideoRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "缺少必需参数: url",
		})
		return
	}

	if err := validateVideoURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"video_url": req.URL,
		"client_ip": c.ClientIP(),
	}).Info("收到视频信息请求")

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 获取分片总数
	segmentCount, err := h.segmentManager.GetSegmentCount(ctx, req.URL)
	if err != nil {
		logger.Errorf("获取视频信息失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "info_error",
			Message: "获取视频信息失败",
		})
		return
	}

	c.JSON(http.StatusOK, VideoInfoResponse{
		URL:             req.URL,
		SegmentCount:    segmentCount,
		SegmentDuration: h.config.Segment.Duration,
	})
}

// HealthCheck 健康检查
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:  "ok",
		Version: h.version,
		Time:    time.Now().Unix(),
	})
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// VideoInfoResponse 视频信息响应
type VideoInfoResponse struct {
	URL             string `json:"url"`
	SegmentCount    int    `json:"segment_count"`
	SegmentDuration int    `json:"segment_duration"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Time    int64  `json:"time"`
}

// GetSegment 获取指定分片
func (h *Handler) GetSegment(c *gin.Context) {
	videoURL := c.Query("url")
	if videoURL == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "缺少必需参数: url",
		})
		return
	}

	if err := validateVideoURL(videoURL); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	indexStr := c.Query("index")
	if indexStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "缺少必需参数: index",
		})
		return
	}

	index, indexErr := strconv.Atoi(indexStr)
	if indexErr != nil || index < 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "无效的分片索引",
		})
		return
	}

	// 获取并校验格式参数，默认 mp4
	format := c.DefaultQuery("format", "mp4")
	switch format {
	case "mp4", "ts", "hls-fmp4":
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "不支持的分片格式，仅支持 mp4、ts 或 hls-fmp4",
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"video_url":     videoURL,
		"segment_index": index,
		"client_ip":     c.ClientIP(),
	}).Info("收到分片请求")

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.config.GetServerTimeout())
	defer cancel()

	// 获取分片
	var err error
	var reader io.ReadCloser

	// 根据格式选择不同的获取方式
	if format == "mp4" {
		reader, err = h.segmentManager.GetSegment(ctx, videoURL, index)
	} else {
		reader, err = h.segmentManager.GetSegmentWithFormat(ctx, videoURL, index, format)
	}

	if err != nil {
		logger.Errorf("获取视频分片失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "transcode_error",
			Message: "视频转码失败",
		})
		return
	}
	defer reader.Close()

	// 根据格式设置响应头
	contentType := "video/mp4"
	if format == "hls-fmp4" {
		contentType = "video/mp4" // HLS FMP4 仍使用 video/mp4 类型
	} else if format == "ts" {
		contentType = "video/MP2T" // HLS TS 使用不同的类型
	}

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=31536000")

	// 流式传输
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, reader); err != nil {
		logger.Errorf("流式传输失败: %v", err)
	}
}

// GetHLSPlaylist 获取 HLS M3U8 播放列表
func (h *Handler) GetHLSPlaylist(c *gin.Context) {
	videoURL := c.Query("url")
	if videoURL == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "缺少必需参数: url",
		})
		return
	}

	if err := validateVideoURL(videoURL); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"video_url": videoURL,
		"client_ip": c.ClientIP(),
	}).Info("收到 HLS 播放列表请求")

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 生成 M3U8 播放列表
	playlist, err := h.segmentManager.GenerateM3U8(ctx, videoURL)
	if err != nil {
		logger.Errorf("生成 M3U8 失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "generate_m3u8_failed",
			Message: "生成 HLS 播放列表失败",
		})
		return
	}

	// 设置响应头
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.Status(http.StatusOK)
	c.String(http.StatusOK, playlist)
}

// GetHLSSegment 获取 HLS 分片
func (h *Handler) GetHLSSegment(c *gin.Context) {
	videoURL := c.Query("url")
	if videoURL == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "缺少必需参数: url",
		})
		return
	}

	if err := validateVideoURL(videoURL); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	indexStr := c.Query("index")
	if indexStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "缺少必需参数: index",
		})
		return
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "无效的分片索引",
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"video_url":     videoURL,
		"segment_index": index,
		"client_ip":     c.ClientIP(),
	}).Info("收到 HLS 分片请求")

	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.config.GetServerTimeout())
	defer cancel()

	// 获取 ts 格式分片
	reader, err := h.segmentManager.GetSegmentWithFormat(ctx, videoURL, index, "ts")
	if err != nil {
		logger.Errorf("获取 HLS 分片失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "transcode_error",
			Message: "视频转码失败",
		})
		return
	}
	defer reader.Close()

	// 设置响应头
	c.Header("Content-Type", "video/MP2T")
	c.Header("Cache-Control", "public, max-age=31536000")

	// 重要：HLS 流需要正确的 MIME 类型
	// 对于 HLS TS 分片，确保 Content-Type 是 video/MP2T

	// 流式传输
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, reader); err != nil {
		logger.Errorf("流式传输失败: %v", err)
	}
}
