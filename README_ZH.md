# PlugStream

[English](README.md) | 简体中文

[![CI](https://github.com/icestr/PlugStream/actions/workflows/ci.yml/badge.svg)](https://github.com/icestr/PlugStream/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/icestr/PlugStream)](https://github.com/icestr/PlugStream/releases)

基于 Go 和 FFmpeg 的轻量级实时视频转码与流式代理服务。PlugStream 按需将远程 HTTP/HTTPS 视频实时转码为 H.264/AAC 分片，缓存在本地磁盘，并通过 HTTP Range 或 HLS（fMP4 / MPEG-TS）输出给播放器。

## 核心特性

- **按需分片转码**：按固定时长（默认 10 秒）切片，FFmpeg 边转码边通过管道流式输出，无需等待整片转完。
- **HLS 与 MP4 双模式**：支持生成标准 VOD M3U8 播放列表（`#EXT-X-VERSION:7`），支持 `mp4`（fMP4）、`hls-fmp4` 与 `ts` 分片格式。
- **HTTP Range 支持**：支持 `bytes=start-end`、`bytes=start-`、`bytes=-suffix` 三种范围请求。
- **本地分片缓存**：基于视频 URL 的 MD5 分桶目录缓存，带分片级互斥锁防并发重复转码，并按配置周期清理过期文件。
- **并发任务限流**：通过信号量限制同时运行的 FFmpeg 转码任务数，防止服务器资源耗尽。

## 快速开始

### 前置要求

- Go 1.24 或更高版本（源码运行或编译时）
- 系统 `PATH` 中已安装 `ffmpeg` 和 `ffprobe`（或在 `config.yaml` 中指定 `ffmpeg.binary_path`）

### 方式 1：源码运行

```bash
git clone https://github.com/icestr/PlugStream.git
cd PlugStream
make install-deps
make run
```

### 方式 2：Docker Compose

```bash
git clone https://github.com/icestr/PlugStream.git
cd PlugStream
docker compose up -d
```

### 方式 3：编译二进制运行

```bash
make build
./build/plugstream -config config.yaml
```

验证服务是否启动成功：

```bash
curl http://localhost:26480/api/v1/health
```

## HTTP 接口一览

基础路径：`http://localhost:26480/api/v1`

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/health` | 服务健康检查与版本查询 |
| `GET` | `/video/info?url=<video_url>` | 探测视频时长并计算总分片数 |
| `GET` | `/video/stream?url=<video_url>` | 获取视频流（支持 `Range` 请求） |
| `GET` | `/video/segment?url=<video_url>&index=<n>&format=<mp4\|ts\|hls-fmp4>` | 获取指定索引与格式的视频分片 |
| `GET` | `/video/hls/playlist?url=<video_url>` | 生成 HLS M3U8 播放列表（`hls-fmp4`） |
| `GET` | `/video/hls/segment?url=<video_url>&index=<n>` | 获取 HLS MPEG-TS 分片 |

调用示例：

```bash
# 获取 HLS 播放列表
curl "http://localhost:26480/api/v1/video/hls/playlist?url=https://example.com/video.mp4"

# 获取第 0 个 hls-fmp4 分片
curl "http://localhost:26480/api/v1/video/segment?url=https://example.com/video.mp4&index=0&format=hls-fmp4" -o seg0.mp4
```

仓库内置了一个可直接使用的网页播放器示例：[`examples/player.html`](examples/player.html)。

## 配置说明

配置文件位于 [`config.yaml`](config.yaml)：

- `server`：监听地址与端口、请求超时（秒）、最大并发转码任务数 `max_concurrent_tasks`。
- `ffmpeg`：可选 `binary_path`、视频/音频编码器、码率、`preset` 和 `crf`。
- `segment`：分片时长 `duration`（秒）与默认容器格式 `format`（`fmp4`）。
- `cache`：缓存目录 `directory`、过期时间 `expiration`（小时）、容量上限 `max_size`（GB）与清理间隔 `cleanup_interval`（小时）。
- `log`：日志级别 `level`（`debug`/`info`/`warn`/`error`）、格式 `format`（`json`/`text`）与可选输出文件 `output`。

## 相关文档

- [使用指南](docs/USAGE.md)
- [API 接口文档](docs/API.md)
- [架构设计](docs/ARCHITECTURE.md)
- [贡献指南](CONTRIBUTING.md)
- [安全策略](SECURITY.md)
- [更新日志](CHANGELOG.md)

## 安全提示

PlugStream 会将请求参数中的 `http`/`https` 视频地址直接交给 `ffmpeg` 与 `ffprobe` 访问。请勿在未做鉴权、限流和源站域名白名单限制的情况下将实例直接暴露到公网。详情请查阅 [SECURITY.md](SECURITY.md)。

## 许可证

本项目基于 [MIT License](LICENSE) 开源。
