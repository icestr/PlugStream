# PlugStream 使用指南

## 前置要求

- **源码构建 / 运行**：Go 1.24 或更高版本、`ffmpeg` 与 `ffprobe`（需在 `PATH` 中，或在 `config.yaml` 指定路径）。
- **Docker 运行**：Docker 与 Docker Compose（镜像内已包含 `ffmpeg`）。

## 安装与启动

### 方式 1：从源码运行

```bash
git clone https://github.com/icestr/PlugStream.git
cd PlugStream

# 下载并整理依赖
make install-deps

# 直接运行（使用当前目录 config.yaml）
make run
```

或编译为二进制文件后运行：

```bash
make build
./build/plugstream -config config.yaml
```

### 方式 2：使用 Docker Compose

```bash
git clone https://github.com/icestr/PlugStream.git
cd PlugStream

docker compose up -d
docker compose logs -f plugstream
```

### 方式 3：手动构建 Docker 镜像

```bash
make docker-build
make docker-run
```

## 配置文件说明

默认配置文件为仓库根目录的 `config.yaml`：

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  timeout: 300              # 请求超时时间（秒）
  max_concurrent_tasks: 10  # 最大并发转码任务数

ffmpeg:
  binary_path: ""           # 留空时使用系统 PATH 中的 ffmpeg/ffprobe
  video_codec: "libx264"
  audio_codec: "aac"
  video_bitrate: "2M"
  audio_bitrate: "128k"
  preset: "veryfast"        # ultrafast, superfast, veryfast, faster, fast, medium, slow...
  crf: 23                   # 0-51，越小画质越好

segment:
  duration: 10              # 单片时长（秒）
  format: "fmp4"            # 当前配置校验仅支持 fmp4

cache:
  directory: "./cache"
  expiration: 24            # 过期时间（小时）
  max_size: 50              # 容量告警阈值（GB）
  cleanup_interval: 1       # 清理检查间隔（小时）

log:
  level: "info"             # debug, info, warn, error
  format: "json"            # json, text
  output: ""                # 留空时输出到标准输出
```

## 常见用法

### 1. 健康检查

```bash
curl http://localhost:8080/api/v1/health
```

### 2. 使用 HLS 播放（推荐）

配合 `hls.js` 或原生支持 HLS 的浏览器，直接加载 M3U8 播放列表即可实现进度拖动与按需切片转码：

```html
<video id="player" controls></video>
<script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
<script>
  const video = document.getElementById('player');
  const videoUrl = 'https://example.com/video.mp4';
  const m3u8Url = `http://localhost:8080/api/v1/video/hls/playlist?url=${encodeURIComponent(videoUrl)}`;

  if (Hls.isSupported()) {
    const hls = new Hls();
    hls.loadSource(m3u8Url);
    hls.attachMedia(video);
  } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
    video.src = m3u8Url;
  }
</script>
```

也可以直接用浏览器打开仓库内的 [`examples/player.html`](../examples/player.html) 进行本地调试。

### 3. 按分片或流式接口拉取

```bash
# 获取视频总分片数
curl "http://localhost:8080/api/v1/video/info?url=https://example.com/video.mp4"

# 获取第 0 个分片（格式可选 mp4 / ts / hls-fmp4）
curl "http://localhost:8080/api/v1/video/segment?url=https://example.com/video.mp4&index=0&format=hls-fmp4" -o seg0.mp4
```

详细参数和错误码说明见 [API 文档](./API.md)。

## 调优与生产部署建议

1. **并发控制**：`server.max_concurrent_tasks` 建议按 CPU 核心数设置（如 4 核机器设为 `4` 至 `6`），避免同时拉起过多 FFmpeg 进程拖垮主机。
2. **转码速度与画质**：优先保证首屏速度时可使用 `preset: "ultrafast"`、适当提高 `crf`（如 `26`）；追求画质可调至 `faster` / `medium`。
3. **网关与安全防护**：生产环境建议在 Nginx / Caddy 后运行并开启 HTTPS；同时在网关层增加鉴权、请求限流，并限制允许转码的视频源域名。

## 故障排查

- **`FFmpeg 不可用`**：确认本机已安装 `ffmpeg` 与 `ffprobe`，或在 `config.yaml` 的 `ffmpeg.binary_path` 填入完整路径。
- **端口占用**：修改 `config.yaml` 中的 `server.port`。
- **转码耗时高或失败**：将 `log.level` 改为 `debug` 查看 FFmpeg 标准错误输出，并检查源视频 URL 是否可从当前服务器直接访问。
