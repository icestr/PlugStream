package config

import (
	"os"
	"testing"
)

// TestLoadConfig 测试加载配置
func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	configContent := `
server:
  host: "0.0.0.0"
  port: 8080
  timeout: 300
  max_concurrent_tasks: 10

ffmpeg:
  binary_path: ""
  video_codec: "libx264"
  audio_codec: "aac"
  video_bitrate: "2M"
  audio_bitrate: "128k"
  preset: "veryfast"
  crf: 23

segment:
  duration: 10
  format: "fmp4"

cache:
  directory: "./cache"
  expiration: 24
  max_size: 50
  cleanup_interval: 1

log:
  level: "info"
  format: "json"
  output: ""
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}
	tmpFile.Close()

	// 加载配置
	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证配置
	if cfg.Server.Port != 8080 {
		t.Errorf("端口配置错误: 期望 8080, 实际 %d", cfg.Server.Port)
	}

	if cfg.FFmpeg.VideoCodec != "libx264" {
		t.Errorf("视频编码器配置错误: 期望 libx264, 实际 %s", cfg.FFmpeg.VideoCodec)
	}

	if cfg.Segment.Duration != 10 {
		t.Errorf("分片时长配置错误: 期望 10, 实际 %d", cfg.Segment.Duration)
	}
}

// TestValidateConfig 测试配置验证
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "有效配置",
			config: Config{
				Server: ServerConfig{
					Port:               8080,
					Timeout:            300,
					MaxConcurrentTasks: 10,
				},
				FFmpeg: FFmpegConfig{
					VideoCodec: "libx264",
					AudioCodec: "aac",
					CRF:        23,
				},
				Segment: SegmentConfig{
					Duration: 10,
					Format:   "fmp4",
				},
				Cache: CacheConfig{
					Directory:  "./cache",
					Expiration: 24,
					MaxSize:    50,
				},
				Log: LogConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: false,
		},
		{
			name: "无效端口",
			config: Config{
				Server: ServerConfig{
					Port:               0,
					Timeout:            300,
					MaxConcurrentTasks: 10,
				},
				FFmpeg: FFmpegConfig{
					VideoCodec: "libx264",
					AudioCodec: "aac",
					CRF:        23,
				},
				Segment: SegmentConfig{
					Duration: 10,
					Format:   "fmp4",
				},
				Cache: CacheConfig{
					Directory:  "./cache",
					Expiration: 24,
					MaxSize:    50,
				},
				Log: LogConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
		},
		{
			name: "无效 CRF",
			config: Config{
				Server: ServerConfig{
					Port:               8080,
					Timeout:            300,
					MaxConcurrentTasks: 10,
				},
				FFmpeg: FFmpegConfig{
					VideoCodec: "libx264",
					AudioCodec: "aac",
					CRF:        100,
				},
				Segment: SegmentConfig{
					Duration: 10,
					Format:   "fmp4",
				},
				Cache: CacheConfig{
					Directory:  "./cache",
					Expiration: 24,
					MaxSize:    50,
				},
				Log: LogConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
		},
		{
			name: "无效分片格式",
			config: Config{
				Server: ServerConfig{
					Port:               8080,
					Timeout:            300,
					MaxConcurrentTasks: 10,
				},
				FFmpeg: FFmpegConfig{
					VideoCodec: "libx264",
					AudioCodec: "aac",
					CRF:        23,
				},
				Segment: SegmentConfig{
					Duration: 10,
					Format:   "mp4",
				},
				Cache: CacheConfig{
					Directory:  "./cache",
					Expiration: 24,
					MaxSize:    50,
				},
				Log: LogConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetServerAddress 测试获取服务器地址
func TestGetServerAddress(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
	}

	addr := cfg.GetServerAddress()
	expected := "0.0.0.0:8080"

	if addr != expected {
		t.Errorf("GetServerAddress() = %s, 期望 %s", addr, expected)
	}
}
