package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	FFmpeg  FFmpegConfig  `mapstructure:"ffmpeg"`
	Segment SegmentConfig `mapstructure:"segment"`
	Cache   CacheConfig   `mapstructure:"cache"`
	Log     LogConfig     `mapstructure:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host               string `mapstructure:"host"`
	Port               int    `mapstructure:"port"`
	Timeout            int    `mapstructure:"timeout"`
	MaxConcurrentTasks int    `mapstructure:"max_concurrent_tasks"`
}

// FFmpegConfig FFmpeg 配置
type FFmpegConfig struct {
	BinaryPath   string `mapstructure:"binary_path"`
	VideoCodec   string `mapstructure:"video_codec"`
	AudioCodec   string `mapstructure:"audio_codec"`
	VideoBitrate string `mapstructure:"video_bitrate"`
	AudioBitrate string `mapstructure:"audio_bitrate"`
	Preset       string `mapstructure:"preset"`
	CRF          int    `mapstructure:"crf"`
}

// SegmentConfig 分片配置
type SegmentConfig struct {
	Duration int    `mapstructure:"duration"`
	Format   string `mapstructure:"format"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Directory       string `mapstructure:"directory"`
	Expiration      int    `mapstructure:"expiration"`
	MaxSize         int    `mapstructure:"max_size"`
	CleanupInterval int    `mapstructure:"cleanup_interval"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// globalConfig 全局配置实例
var globalConfig *Config

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// 默认配置文件路径
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	globalConfig = &cfg
	return &cfg, nil
}

// Get 获取全局配置
func Get() *Config {
	return globalConfig
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	// 验证服务器配置
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("无效的端口号: %d", c.Server.Port)
	}
	if c.Server.Timeout <= 0 {
		return fmt.Errorf("无效的超时时间: %d", c.Server.Timeout)
	}
	if c.Server.MaxConcurrentTasks <= 0 {
		return fmt.Errorf("无效的最大并发任务数: %d", c.Server.MaxConcurrentTasks)
	}

	// 验证 FFmpeg 配置
	if c.FFmpeg.VideoCodec == "" {
		return fmt.Errorf("视频编码器不能为空")
	}
	if c.FFmpeg.AudioCodec == "" {
		return fmt.Errorf("音频编码器不能为空")
	}
	if c.FFmpeg.CRF < 0 || c.FFmpeg.CRF > 51 {
		return fmt.Errorf("无效的 CRF 值: %d (有效范围: 0-51)", c.FFmpeg.CRF)
	}

	// 验证分片配置
	if c.Segment.Duration <= 0 {
		return fmt.Errorf("无效的分片时长: %d", c.Segment.Duration)
	}
	if c.Segment.Format != "fmp4" {
		return fmt.Errorf("不支持的分片格式: %s (仅支持 fmp4)", c.Segment.Format)
	}

	// 验证缓存配置
	if c.Cache.Directory == "" {
		return fmt.Errorf("缓存目录不能为空")
	}
	if c.Cache.Expiration <= 0 {
		return fmt.Errorf("无效的缓存过期时间: %d", c.Cache.Expiration)
	}
	if c.Cache.MaxSize <= 0 {
		return fmt.Errorf("无效的最大缓存大小: %d", c.Cache.MaxSize)
	}

	// 验证日志配置
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.Log.Level] {
		return fmt.Errorf("无效的日志级别: %s", c.Log.Level)
	}
	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[c.Log.Format] {
		return fmt.Errorf("无效的日志格式: %s", c.Log.Format)
	}

	return nil
}

// GetServerAddress 获取服务器监听地址
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetServerTimeout 获取服务器超时时间
func (c *Config) GetServerTimeout() time.Duration {
	return time.Duration(c.Server.Timeout) * time.Second
}

// GetCacheExpiration 获取缓存过期时间
func (c *Config) GetCacheExpiration() time.Duration {
	return time.Duration(c.Cache.Expiration) * time.Hour
}

// GetCleanupInterval 获取清理间隔
func (c *Config) GetCleanupInterval() time.Duration {
	return time.Duration(c.Cache.CleanupInterval) * time.Hour
}

// GetMaxCacheSizeBytes 获取最大缓存大小（字节）
func (c *Config) GetMaxCacheSizeBytes() int64 {
	return int64(c.Cache.MaxSize) * 1024 * 1024 * 1024 // GB to Bytes
}
