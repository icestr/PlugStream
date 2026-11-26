package segment

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/icestr/PlugStream/internal/config"
	"github.com/icestr/PlugStream/internal/logger"
	"github.com/icestr/PlugStream/internal/transcoder"
)

// Manager 分片管理器接口
type Manager interface {
	// GetSegment 获取指定的视频分片
	GetSegment(ctx context.Context, videoURL string, segmentIndex int) (io.ReadCloser, error)
	// GetSegmentWithFormat 获取指定格式的视频分片
	GetSegmentWithFormat(ctx context.Context, videoURL string, segmentIndex int, format string) (io.ReadCloser, error)
	// GetSegmentPath 获取分片的缓存路径
	GetSegmentPath(videoURL string, segmentIndex int) string
	// CalculateSegmentIndex 根据时间计算分片索引
	CalculateSegmentIndex(timeInSeconds int) int
	// GetSegmentCount 获取视频的总分片数
	GetSegmentCount(ctx context.Context, videoURL string) (int, error)
	// GenerateM3U8 生成 HLS M3U8 播放列表
	GenerateM3U8(ctx context.Context, videoURL string) (string, error)
	// Close 关闭管理器
	Close() error
}

// GenerateM3U8 generates HLS M3U8 playlist content
func (m *SegmentManager) GenerateM3U8(ctx context.Context, videoURL string) (string, error) {
	// Get video duration
	info, err := m.transcoder.GetVideoInfo(ctx, videoURL)
	if err != nil {
		return "", fmt.Errorf("get video info failed: %w", err)
	}

	// Calculate total segments
	totalSegments := m.calculateSegmentCount(info.Duration)
	segmentDuration := m.config.Segment.Duration

	// 生成 HLS FMP4 播放列表内容
	var m3u8 strings.Builder
	m3u8.WriteString("#EXTM3U\n")
	m3u8.WriteString("#EXT-X-VERSION:7\n")
	m3u8.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", segmentDuration+2))
	m3u8.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	m3u8.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	m3u8.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")

	for i := 0; i < totalSegments; i++ {
		var actualDuration float64
		if i == totalSegments-1 && totalSegments > 0 {
			actualDuration = info.Duration - (float64(i) * float64(segmentDuration))
			if actualDuration <= 0 {
				actualDuration = float64(segmentDuration)
			}
		} else {
			actualDuration = float64(segmentDuration)
		}

		m3u8.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", actualDuration))
		m3u8.WriteString(fmt.Sprintf("/api/v1/video/segment?url=%s&index=%d&format=hls-fmp4\n",
			url.QueryEscape(videoURL), i))
	}

	m3u8.WriteString("#EXT-X-ENDLIST\n")

	return m3u8.String(), nil
}

// SegmentManager 分片管理器实现
type SegmentManager struct {
	config     *config.Config
	transcoder transcoder.Transcoder
	// 分片缓存锁，防止同一分片被多次转码
	segmentLocks sync.Map // key: segmentKey, value: *sync.Mutex
	// 视频信息缓存
	videoInfoCache sync.Map // key: videoURL, value: *transcoder.VideoInfo
}

// NewSegmentManager 创建分片管理器
func NewSegmentManager(cfg *config.Config, trans transcoder.Transcoder) (*SegmentManager, error) {
	// 确保缓存目录存在
	if err := os.MkdirAll(cfg.Cache.Directory, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	manager := &SegmentManager{
		config:     cfg,
		transcoder: trans,
	}

	// 启动缓存清理协程
	go manager.startCacheCleanup()

	return manager, nil
}

// GetSegment 获取指定的视频分片
func (m *SegmentManager) GetSegment(ctx context.Context, videoURL string, segmentIndex int) (io.ReadCloser, error) {
	// 生成分片缓存路径
	segmentPath := m.GetSegmentPath(videoURL, segmentIndex)

	// 检查缓存是否存在
	if _, err := os.Stat(segmentPath); err == nil {
		logger.WithFields(map[string]interface{}{
			"video_url":     videoURL,
			"segment_index": segmentIndex,
			"cache_path":    segmentPath,
		}).Info("使用缓存分片")

		// 从缓存读取
		file, err := os.Open(segmentPath)
		if err != nil {
			return nil, fmt.Errorf("打开缓存文件失败: %w", err)
		}
		return file, nil
	}

	// 获取分片锁，防止重复转码
	segmentKey := m.getSegmentKey(videoURL, segmentIndex)
	lockValue, _ := m.segmentLocks.LoadOrStore(segmentKey, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)

	lock.Lock()
	defer lock.Unlock()

	// 再次检查缓存（可能在等待锁期间已被其他协程创建）
	if _, err := os.Stat(segmentPath); err == nil {
		file, err := os.Open(segmentPath)
		if err != nil {
			return nil, fmt.Errorf("打开缓存文件失败: %w", err)
		}
		return file, nil
	}

	logger.WithFields(map[string]interface{}{
		"video_url":     videoURL,
		"segment_index": segmentIndex,
	}).Info("开始转码分片")

	// 计算分片的起始时间和时长
	startTime := segmentIndex * m.config.Segment.Duration
	duration := m.config.Segment.Duration

	// 转码分片
	reader, err := m.transcoder.TranscodeSegment(ctx, videoURL, startTime, duration)
	if err != nil {
		return nil, fmt.Errorf("转码分片失败: %w", err)
	}

	// 创建临时文件
	tempPath := segmentPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		reader.Close()
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}

	// 使用 TeeReader 同时写入文件和返回给客户端
	teeReader := io.TeeReader(reader, tempFile)

	// 返回一个包装的 ReadCloser，在关闭时完成文件写入
	return &segmentReader{
		reader:      teeReader,
		transcoder:  reader,
		tempFile:    tempFile,
		tempPath:    tempPath,
		segmentPath: segmentPath,
	}, nil
}

// GetSegmentWithFormat 获取指定格式的视频分片
func (m *SegmentManager) GetSegmentWithFormat(ctx context.Context, videoURL string, segmentIndex int, format string) (io.ReadCloser, error) {
	// 生成分片缓存路径
	segmentPath := m.GetSegmentPathWithFormat(videoURL, segmentIndex, format)

	// 检查缓存是否存在
	if _, err := os.Stat(segmentPath); err == nil {
		logger.WithFields(map[string]interface{}{
			"video_url":     videoURL,
			"segment_index": segmentIndex,
			"format":        format,
			"cache_path":    segmentPath,
		}).Info("使用缓存分片")

		// 从缓存读取
		file, err := os.Open(segmentPath)
		if err != nil {
			return nil, fmt.Errorf("打开缓存文件失败: %w", err)
		}
		return file, nil
	}

	// 获取分片锁，防止重复转码
	segmentKey := m.getSegmentKeyWithFormat(videoURL, segmentIndex, format)
	lockValue, _ := m.segmentLocks.LoadOrStore(segmentKey, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)

	lock.Lock()
	defer lock.Unlock()

	// 再次检查缓存（可能在等待锁期间已被其他协程创建）
	if _, err := os.Stat(segmentPath); err == nil {
		file, err := os.Open(segmentPath)
		if err != nil {
			return nil, fmt.Errorf("打开缓存文件失败: %w", err)
		}
		return file, nil
	}

	logger.WithFields(map[string]interface{}{
		"video_url":     videoURL,
		"segment_index": segmentIndex,
		"format":        format,
	}).Info("开始转码分片")

	// 计算分片的起始时间和时长
	startTime := segmentIndex * m.config.Segment.Duration
	duration := m.config.Segment.Duration

	// 转码分片
	reader, err := m.transcoder.TranscodeSegmentWithFormat(ctx, videoURL, startTime, duration, format)
	if err != nil {
		return nil, fmt.Errorf("转码分片失败: %w", err)
	}

	// 创建临时文件
	tempPath := segmentPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		reader.Close()
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}

	// 使用 TeeReader 同时写入文件和返回给客户端
	teeReader := io.TeeReader(reader, tempFile)

	// 返回一个包装的 ReadCloser，在关闭时完成文件写入
	return &segmentReader{
		reader:      teeReader,
		transcoder:  reader,
		tempFile:    tempFile,
		tempPath:    tempPath,
		segmentPath: segmentPath,
	}, nil
}

// GetSegmentPath 获取分片的缓存路径
func (m *SegmentManager) GetSegmentPath(videoURL string, segmentIndex int) string {
	// 使用 MD5 哈希生成唯一的文件名
	hash := md5.Sum([]byte(videoURL))
	hashStr := hex.EncodeToString(hash[:])

	// 创建子目录（使用哈希的前两个字符）
	subDir := hashStr[:2]
	dir := filepath.Join(m.config.Cache.Directory, subDir)

	// 确保目录存在
	os.MkdirAll(dir, 0755)

	// 生成文件名
	filename := fmt.Sprintf("%s_seg_%d.mp4", hashStr, segmentIndex)
	return filepath.Join(dir, filename)
}

// CalculateSegmentIndex 根据时间计算分片索引
func (m *SegmentManager) CalculateSegmentIndex(timeInSeconds int) int {
	return timeInSeconds / m.config.Segment.Duration
}

// GetSegmentCount 获取视频的总分片数
func (m *SegmentManager) GetSegmentCount(ctx context.Context, videoURL string) (int, error) {
	// 尝试从缓存获取视频信息
	if cached, ok := m.videoInfoCache.Load(videoURL); ok {
		info := cached.(*transcoder.VideoInfo)
		return m.calculateSegmentCount(info.Duration), nil
	}

	// 获取视频信息
	info, err := m.transcoder.GetVideoInfo(ctx, videoURL)
	if err != nil {
		return 0, fmt.Errorf("获取视频信息失败: %w", err)
	}

	// 缓存视频信息
	m.videoInfoCache.Store(videoURL, info)

	return m.calculateSegmentCount(info.Duration), nil
}

// Close 关闭管理器
func (m *SegmentManager) Close() error {
	return nil
}

// getSegmentKey 生成分片的唯一键
func (m *SegmentManager) getSegmentKey(videoURL string, segmentIndex int) string {
	return fmt.Sprintf("%s:%d", videoURL, segmentIndex)
}

// getSegmentKeyWithFormat 生成带格式的分片唯一键
func (m *SegmentManager) getSegmentKeyWithFormat(videoURL string, segmentIndex int, format string) string {
	return fmt.Sprintf("%s:%d:%s", videoURL, segmentIndex, format)
}

// GetSegmentPathWithFormat 获取指定格式分片的缓存路径
func (m *SegmentManager) GetSegmentPathWithFormat(videoURL string, segmentIndex int, format string) string {
	// 使用 MD5 哈希生成唯一的文件名
	hash := md5.Sum([]byte(videoURL))
	hashStr := hex.EncodeToString(hash[:])

	// 创建子目录（使用哈希的前两个字符）
	subDir := hashStr[:2]
	dir := filepath.Join(m.config.Cache.Directory, subDir)

	// 确保目录存在
	os.MkdirAll(dir, 0755)

	// 生成文件名
	filename := fmt.Sprintf("%s_seg_%d.%s", hashStr, segmentIndex, format)
	return filepath.Join(dir, filename)
}

// calculateSegmentCount 计算分片总数
func (m *SegmentManager) calculateSegmentCount(duration float64) int {
	segmentDuration := float64(m.config.Segment.Duration)
	count := int(duration / segmentDuration)
	if duration > float64(count)*segmentDuration {
		count++
	}
	return count
}

// startCacheCleanup 启动缓存清理协程
func (m *SegmentManager) startCacheCleanup() {
	ticker := time.NewTicker(m.config.GetCleanupInterval())
	defer ticker.Stop()

	for range ticker.C {
		if err := m.cleanupCache(); err != nil {
			logger.Errorf("清理缓存失败: %v", err)
		}
	}
}

// cleanupCache 清理过期缓存
func (m *SegmentManager) cleanupCache() error {
	logger.Info("开始清理过期缓存")

	expiration := m.config.GetCacheExpiration()
	now := time.Now()
	var totalSize int64
	var deletedCount int

	// 遍历缓存目录
	err := filepath.Walk(m.config.Cache.Directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 跳过临时文件
		if filepath.Ext(path) == ".tmp" {
			return nil
		}

		// 检查文件是否过期
		if now.Sub(info.ModTime()) > expiration {
			if err := os.Remove(path); err != nil {
				logger.Warnf("删除过期缓存文件失败: %s, 错误: %v", path, err)
			} else {
				deletedCount++
				logger.Debugf("删除过期缓存文件: %s", path)
			}
		} else {
			totalSize += info.Size()
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("遍历缓存目录失败: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"deleted_count": deletedCount,
		"total_size_mb": totalSize / 1024 / 1024,
	}).Info("缓存清理完成")

	// 检查缓存大小是否超限
	maxSize := m.config.GetMaxCacheSizeBytes()
	if totalSize > maxSize {
		logger.Warnf("缓存大小超限: %d MB / %d MB", totalSize/1024/1024, maxSize/1024/1024)
		// 这里可以实现 LRU 清理策略
	}

	return nil
}

// segmentReader 分片读取器，在关闭时完成文件写入
type segmentReader struct {
	reader      io.Reader
	transcoder  io.ReadCloser
	tempFile    *os.File
	tempPath    string
	segmentPath string
	once        sync.Once
}

// Read 实现 io.Reader 接口
func (r *segmentReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

// Close 实现 io.Closer 接口
func (r *segmentReader) Close() error {
	var closeErr error
	r.once.Do(func() {
		// 关闭转码器
		if err := r.transcoder.Close(); err != nil {
			logger.Errorf("关闭转码器失败: %v", err)
			closeErr = err
		}

		// 关闭临时文件
		if err := r.tempFile.Close(); err != nil {
			logger.Errorf("关闭临时文件失败: %v", err)
			if closeErr == nil {
				closeErr = err
			}
		}

		// 将临时文件重命名为正式文件
		if err := os.Rename(r.tempPath, r.segmentPath); err != nil {
			logger.Errorf("重命名缓存文件失败: %v", err)
			// 清理临时文件
			os.Remove(r.tempPath)
			if closeErr == nil {
				closeErr = err
			}
		} else {
			logger.Infof("分片缓存成功: %s", r.segmentPath)
		}
	})

	return closeErr
}
