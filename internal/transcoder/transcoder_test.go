package transcoder

import (
	"context"
	"testing"
	"time"

	"github.com/icestr/PlugStream/internal/config"
)

// TestNewFFmpegTranscoder 测试创建转码器
func TestNewFFmpegTranscoder(t *testing.T) {
	cfg := &config.Config{
		FFmpeg: config.FFmpegConfig{
			BinaryPath:   "",
			VideoCodec:   "libx264",
			AudioCodec:   "aac",
			VideoBitrate: "2M",
			AudioBitrate: "128k",
			Preset:       "veryfast",
			CRF:          23,
		},
		Server: config.ServerConfig{
			MaxConcurrentTasks: 10,
		},
	}

	transcoder, err := NewFFmpegTranscoder(cfg)
	if err != nil {
		t.Skipf("跳过测试: FFmpeg 不可用 - %v", err)
		return
	}
	defer transcoder.Close()

	if transcoder == nil {
		t.Fatal("转码器创建失败")
	}
}

// TestBuildFFmpegArgs 测试构建 FFmpeg 参数
func TestBuildFFmpegArgs(t *testing.T) {
	cfg := &config.Config{
		FFmpeg: config.FFmpegConfig{
			VideoCodec:   "libx264",
			AudioCodec:   "aac",
			VideoBitrate: "2M",
			AudioBitrate: "128k",
			Preset:       "veryfast",
			CRF:          23,
		},
		Server: config.ServerConfig{
			MaxConcurrentTasks: 10,
		},
	}

	transcoder := &FFmpegTranscoder{
		config: cfg,
	}

	baseArgs := []string{
		"-y",
		"-ss", "0",
		"-i", "test.mp4",
		"-t", "10",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-b:v", "2M",
		"-c:a", "aac",
		"-b:a", "128k",
	}

	tests := []struct {
		name     string
		format   string
		tailArgs []string
	}{
		{
			name:   "mp4",
			format: "mp4",
			tailArgs: []string{
				"-movflags", "frag_keyframe+empty_moov+default_base_moof",
				"-f", "mp4",
				"pipe:1",
			},
		},
		{
			name:   "ts",
			format: "ts",
			tailArgs: []string{
				"-bsf:v", "h264_mp4toannexb",
				"-f", "mpegts",
				"pipe:1",
			},
		},
		{
			name:   "hls-fmp4",
			format: "hls-fmp4",
			tailArgs: []string{
				"-movflags", "frag_keyframe+empty_moov+default_base_moof",
				"-f", "mp4",
				"-fflags", "+genpts",
				"-reset_timestamps", "1",
				"pipe:1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := transcoder.buildFFmpegArgs("test.mp4", 0, 10, tt.format)
			expectedArgs := append(append([]string{}, baseArgs...), tt.tailArgs...)

			if len(args) != len(expectedArgs) {
				t.Fatalf("参数数量不匹配: 期望 %d, 实际 %d (%v)", len(expectedArgs), len(args), args)
			}

			for i, expected := range expectedArgs {
				if args[i] != expected {
					t.Errorf("参数 %d 不匹配: 期望 %s, 实际 %s", i, expected, args[i])
				}
			}
		})
	}
}

// TestTranscodeSegmentContext 测试上下文取消
func TestTranscodeSegmentContext(t *testing.T) {
	cfg := &config.Config{
		FFmpeg: config.FFmpegConfig{
			BinaryPath:   "",
			VideoCodec:   "libx264",
			AudioCodec:   "aac",
			VideoBitrate: "2M",
			AudioBitrate: "128k",
			Preset:       "veryfast",
			CRF:          23,
		},
		Server: config.ServerConfig{
			MaxConcurrentTasks: 10,
		},
	}

	transcoder, err := NewFFmpegTranscoder(cfg)
	if err != nil {
		t.Skipf("跳过测试: FFmpeg 不可用 - %v", err)
		return
	}
	defer transcoder.Close()

	// 创建一个会立即取消的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// 等待上下文超时
	time.Sleep(10 * time.Millisecond)

	// 尝试转码（应该失败）
	_, err = transcoder.TranscodeSegment(ctx, "test.mp4", 0, 10)
	if err == nil {
		t.Error("期望上下文取消错误，但没有收到错误")
	}
}
