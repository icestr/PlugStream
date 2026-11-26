# 更新日志

本文件记录 PlugStream 的重要变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 计划中
- 硬件加速支持（NVENC / QSV / VAAPI）
- 缓存容量超限时的 LRU 淘汰策略

## [0.1.0] - 2026-11-26

### 新增
- 基于 Go 1.24 与 FFmpeg 的实时视频分片转码服务。
- 支持 `mp4`（fMP4）、`hls-fmp4` 与 `ts`（MPEG-TS）分片输出。
- 新增 HLS M3U8 播放列表接口（`/api/v1/video/hls/playlist`）与 HLS TS 分片接口（`/api/v1/video/hls/segment`）。
- 支持 HTTP `Range` 请求（`bytes=start-end`、`bytes=start-`、`bytes=-suffix`）。
- 本地 MD5 分桶分片缓存、分片级互斥锁防并发重复转码、定时过期清理。
- 视频源 URL 协议校验（仅允许 `http` / `https`）与分片格式白名单校验。
- 整合版网页播放器示例（`examples/player.html`，支持 HLS / MP4 流 / 按分片播放）。
- Docker 多阶段构建与 Docker Compose 编排配置。
- 中英文双语 README、架构/API/使用文档、安全策略与 GitHub CI/Release 工作流。

### 修复
- 修复 `ginLogger` 中间件因读取未写入的 `start_time` 导致访问日志不输出的问题。
- 修复 `GenerateM3U8` 中的非标准标签为 `#EXT-X-INDEPENDENT-SEGMENTS`。
- 修复 `buildFFmpegArgs` 向标准输出管道 `pipe:1` 输出时使用不兼容的 `faststart` 参数问题。
- 修复 `internal/transcoder/transcoder_test.go` 参数签名不匹配导致的编译失败。

[Unreleased]: https://github.com/icestr/PlugStream/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/icestr/PlugStream/releases/tag/v0.1.0
