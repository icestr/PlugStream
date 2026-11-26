package transcoder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/icestr/PlugStream/internal/config"
	"github.com/icestr/PlugStream/internal/logger"
)

// Transcoder 转码器接口
type Transcoder interface {
	// TranscodeSegment 转码指定时间段的视频片段 (默认MP4格式)
	TranscodeSegment(ctx context.Context, videoURL string, startTime, duration int) (io.ReadCloser, error)
	// TranscodeSegmentWithFormat 转码指定时间段的视频片段并指定格式
	TranscodeSegmentWithFormat(ctx context.Context, videoURL string, startTime, duration int, format string) (io.ReadCloser, error)
	// GetVideoInfo 获取视频信息
	GetVideoInfo(ctx context.Context, videoURL string) (*VideoInfo, error)
	// Close 关闭转码器
	Close() error
}

// VideoInfo 视频信息
type VideoInfo struct {
	Duration   float64 // 视频总时长（秒）
	Width      int     // 视频宽度
	Height     int     // 视频高度
	Bitrate    int64   // 比特率
	VideoCodec string  // 视频编码
	AudioCodec string  // 音频编码
	Format     string  // 容器格式
}

// FFmpegTranscoder FFmpeg 转码器实现
type FFmpegTranscoder struct {
	config *config.Config
	mu     sync.RWMutex
	// 活跃的转码任务计数
	activeTaskCount int
	// 任务信号量，用于限制并发数
	taskSem chan struct{}
}

// NewFFmpegTranscoder 创建 FFmpeg 转码器
func NewFFmpegTranscoder(cfg *config.Config) (*FFmpegTranscoder, error) {
	// 检查 FFmpeg 是否可用
	ffmpegPath := cfg.FFmpeg.BinaryPath
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	cmd := exec.Command(ffmpegPath, "-version")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("FFmpeg 不可用: %w", err)
	}

	return &FFmpegTranscoder{
		config:  cfg,
		taskSem: make(chan struct{}, cfg.Server.MaxConcurrentTasks),
	}, nil
}

// TranscodeSegment 转码指定时间段的视频片段 (默认MP4格式)
func (t *FFmpegTranscoder) TranscodeSegment(ctx context.Context, videoURL string, startTime, duration int) (io.ReadCloser, error) {
	// 调用带格式的转码方法
	return t.TranscodeSegmentWithFormat(ctx, videoURL, startTime, duration, "mp4")
}

// TranscodeSegmentWithFormat 转码指定时间段的视频片段并指定格式
func (t *FFmpegTranscoder) TranscodeSegmentWithFormat(ctx context.Context, videoURL string, startTime, duration int, format string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 获取任务信号量
	select {
	case t.taskSem <- struct{}{}:
		defer func() { <-t.taskSem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	t.mu.Lock()
	t.activeTaskCount++
	taskID := t.activeTaskCount
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		t.activeTaskCount--
		t.mu.Unlock()
	}()

	logger.WithFields(map[string]interface{}{
		"task_id":    taskID,
		"video_url":  videoURL,
		"start_time": startTime,
		"duration":   duration,
	}).Info("开始转码任务")

	// 构建 FFmpeg 命令
	args := t.buildFFmpegArgs(videoURL, startTime, duration, format)

	// 创建命令
	ffmpegPath := t.config.FFmpeg.BinaryPath
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	// 获取标准输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建输出管道失败: %w", err)
	}

	// 获取标准错误管道用于日志
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建错误管道失败: %w", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 FFmpeg 失败: %w", err)
	}

	// 异步读取错误输出
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				logger.WithField("task_id", taskID).Debug(string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	// 返回一个包装的 ReadCloser，确保进程被正确清理
	return &transcoderReader{
		reader: stdout,
		cmd:    cmd,
		taskID: taskID,
	}, nil
}

// GetVideoInfo 获取视频信息
func (t *FFmpegTranscoder) GetVideoInfo(ctx context.Context, videoURL string) (*VideoInfo, error) {
	ffprobePath := "ffprobe"
	if t.config.FFmpeg.BinaryPath != "" {
		// 如果指定了 FFmpeg 路径，尝试在同目录找 ffprobe
		dir := filepath.Dir(t.config.FFmpeg.BinaryPath)
		ffprobePath = filepath.Join(dir, "ffprobe")
	}

	// 使用 ffprobe 获取视频信息
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoURL,
	}

	cmd := exec.CommandContext(ctx, ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取视频信息失败: %w", err)
	}

	// 解析 JSON 输出
	info, err := parseVideoInfo(output)
	if err != nil {
		return nil, fmt.Errorf("解析视频信息失败: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"video_url": videoURL,
		"duration":  info.Duration,
		"width":     info.Width,
		"height":    info.Height,
	}).Info("获取视频信息成功")

	return info, nil
}

// Close 关闭转码器
func (t *FFmpegTranscoder) Close() error {
	// 等待所有任务完成
	for i := 0; i < cap(t.taskSem); i++ {
		t.taskSem <- struct{}{}
	}
	close(t.taskSem)
	return nil
}

// buildFFmpegArgs 构建 FFmpeg 命令参数
func (t *FFmpegTranscoder) buildFFmpegArgs(videoURL string, startTime, duration int, format string) []string {
	args := []string{
		"-y",                           // 覆盖输出文件
		"-ss", strconv.Itoa(startTime), // 起始时间
		"-i", videoURL, // 输入文件
		"-t", strconv.Itoa(duration), // 持续时间
		"-c:v", t.config.FFmpeg.VideoCodec, // 视频编码器
		"-preset", t.config.FFmpeg.Preset, // 编码预设
		"-crf", strconv.Itoa(t.config.FFmpeg.CRF), // 质量参数
		"-b:v", t.config.FFmpeg.VideoBitrate, // 视频比特率
		"-c:a", t.config.FFmpeg.AudioCodec, // 音频编码器
		"-b:a", t.config.FFmpeg.AudioBitrate, // 音频比特率
	}

	// 根据不同格式添加特定参数
	switch format {
	case "mp4":
		args = append(args,
			"-movflags", "frag_keyframe+empty_moov+default_base_moof",
			"-f", "mp4",
		)
	case "ts":
		// 对于 HLS TS 格式，支持 b-frames
		args = append(args, "-bsf:v", "h264_mp4toannexb") // H.264 提取 Annex B 格式
		args = append(args, "-f", "mpegts")
	case "hls-fmp4":
		// 对于 HLS FMP4 (CMAF) 格式
		args = append(args,
			"-movflags", "frag_keyframe+empty_moov+default_base_moof",
			"-f", "mp4",
			"-fflags", "+genpts",
			"-reset_timestamps", "1",
		)
	default:
		args = append(args, "-f", format)
	}

	args = append(args, "pipe:1") // 输出到标准输出

	return args
}

// transcoderReader 转码器读取器，包装标准输出并确保进程清理
type transcoderReader struct {
	reader io.ReadCloser
	cmd    *exec.Cmd
	taskID int
	once   sync.Once
}

// Read 实现 io.Reader 接口
func (r *transcoderReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

// Close 实现 io.Closer 接口
func (r *transcoderReader) Close() error {
	var closeErr error
	r.once.Do(func() {
		// 关闭读取器
		if err := r.reader.Close(); err != nil {
			closeErr = err
		}

		// 等待进程结束（带超时）
		done := make(chan error, 1)
		go func() {
			done <- r.cmd.Wait()
		}()

		select {
		case err := <-done:
			if err != nil && closeErr == nil {
				closeErr = err
			}
		case <-time.After(5 * time.Second):
			// 超时则强制杀死进程
			if err := r.cmd.Process.Kill(); err != nil {
				logger.WithField("task_id", r.taskID).Errorf("强制终止进程失败: %v", err)
			}
			closeErr = fmt.Errorf("进程清理超时")
		}

		logger.WithField("task_id", r.taskID).Info("转码任务完成")
	})

	return closeErr
}

// parseVideoInfo 解析视频信息
func parseVideoInfo(data []byte) (*VideoInfo, error) {
	type ffprobeOutput struct {
		Format struct {
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
			FormatName string `json:"format_name"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}

	var output ffprobeOutput
	err := json.Unmarshal(data, &output)
	if err != nil {
		return nil, err
	}

	// 解析时长
	duration, err := strconv.ParseFloat(output.Format.Duration, 64)
	if err != nil {
		return nil, err
	}

	// 解析比特率
	bitrate, err := strconv.ParseInt(output.Format.BitRate, 10, 64)
	if err != nil {
		bitrate = 0 // 允许比特率解析失败
	}

	// 寻找视频和音频流
	var videoCodec, audioCodec string
	var width, height int

	for _, stream := range output.Streams {
		if stream.CodecType == "video" {
			videoCodec = stream.CodecName
			width = stream.Width
			height = stream.Height
		} else if stream.CodecType == "audio" {
			audioCodec = stream.CodecName
		}
	}

	return &VideoInfo{
		Duration:   duration,
		Width:      width,
		Height:     height,
		Bitrate:    bitrate,
		VideoCodec: videoCodec,
		AudioCodec: audioCodec,
		Format:     output.Format.FormatName,
	}, nil
}
