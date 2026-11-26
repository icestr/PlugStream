package logger

import (
	"io"
	"os"

	"github.com/icestr/PlugStream/internal/config"
	"github.com/sirupsen/logrus"
)

// Logger 全局日志实例
var Logger *logrus.Logger

// Init 初始化日志系统
func Init(cfg *config.Config) error {
	Logger = logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		return err
	}
	Logger.SetLevel(level)

	// 设置日志格式
	if cfg.Log.Format == "json" {
		Logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// 设置日志输出
	if cfg.Log.Output != "" {
		file, err := os.OpenFile(cfg.Log.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		Logger.SetOutput(io.MultiWriter(os.Stdout, file))
	} else {
		Logger.SetOutput(os.Stdout)
	}

	return nil
}

// Debug 输出调试日志
func Debug(args ...interface{}) {
	Logger.Debug(args...)
}

// Debugf 格式化输出调试日志
func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

// Info 输出信息日志
func Info(args ...interface{}) {
	Logger.Info(args...)
}

// Infof 格式化输出信息日志
func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

// Warn 输出警告日志
func Warn(args ...interface{}) {
	Logger.Warn(args...)
}

// Warnf 格式化输出警告日志
func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
}

// Error 输出错误日志
func Error(args ...interface{}) {
	Logger.Error(args...)
}

// Errorf 格式化输出错误日志
func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
}

// Fatal 输出致命错误日志并退出程序
func Fatal(args ...interface{}) {
	Logger.Fatal(args...)
}

// Fatalf 格式化输出致命错误日志并退出程序
func Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
}

// WithField 添加单个字段
func WithField(key string, value interface{}) *logrus.Entry {
	return Logger.WithField(key, value)
}

// WithFields 添加多个字段
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Logger.WithFields(fields)
}
