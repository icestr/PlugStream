# PlugStream API 文档

## 基础信息

- **Base URL**: `http://localhost:26480/api/v1`
- **CORS**: 已启用（`Access-Control-Allow-Origin: *`）
- **URL 校验规则**: 所有接收 `url` 参数的接口仅允许 `http` 或 `https` 协议且包含有效主机名的地址；传入 `file://` 等其他协议将返回 `400 Bad Request`。

---

## 接口列表

### 1. 健康检查

检查服务是否正常运行及当前版本。

```http
GET /api/v1/health
```

**响应示例 (`200 OK`)**

```json
{
  "status": "ok",
  "version": "0.1.0",
  "time": 1730000000
}
```

---

### 2. 获取视频信息

调用 `ffprobe` 探测视频时长，并根据配置的分片时长计算分片总数。

```http
GET /api/v1/video/info?url={video_url}
```

**查询参数**

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 视频源 URL（仅限 `http` / `https`） |

**响应示例 (`200 OK`)**

```json
{
  "url": "https://example.com/video.mp4",
  "segment_count": 120,
  "segment_duration": 10
}
```

---

### 3. 获取视频流

获取转码后的 fMP4 视频流，支持 HTTP `Range` 请求。

```http
GET /api/v1/video/stream?url={video_url}
```

**查询参数**

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 视频源 URL（仅限 `http` / `https`） |

**请求头**

| 请求头 | 说明 |
|--------|------|
| `Range` | 可选。支持 `bytes=start-end`、`bytes=start-`、`bytes=-suffix` |

**说明**

- 不带 `Range` 请求头时，从第 `0` 个分片开始流式返回（`200 OK`，`Content-Type: video/mp4`）。
- 携带 `Range` 请求头时，服务端按配置的 `segment.duration` 与估算码率 `2Mbps` 定位到对应分片索引：
  - 若目标分片已缓存，直接读取本地文件并按实际字节范围返回 `206 Partial Content` 与 `Content-Range`。
  - 若目标分片尚未缓存，触发实时转码并在 `206 Partial Content` 响应中流式返回整片内容。

**示例**

```bash
curl "http://localhost:26480/api/v1/video/stream?url=https://example.com/video.mp4" -o stream.mp4
curl -H "Range: bytes=0-1048575" "http://localhost:26480/api/v1/video/stream?url=https://example.com/video.mp4" -o part.mp4
```

---

### 4. 获取指定分片

按分片索引和指定格式获取转码后的视频分片。

```http
GET /api/v1/video/segment?url={video_url}&index={segment_index}&format={format}
```

**查询参数**

| 参数 | 类型 | 必需 | 默认值 | 说明 |
|------|------|------|--------|------|
| `url` | string | 是 | - | 视频源 URL（仅限 `http` / `https`） |
| `index` | int | 是 | - | 分片索引（从 `0` 开始的非负整数） |
| `format` | string | 否 | `mp4` | 分片格式，仅允许 `mp4`、`ts`、`hls-fmp4` |

**响应头**

- `format=mp4` 或 `format=hls-fmp4`：`Content-Type: video/mp4`
- `format=ts`：`Content-Type: video/MP2T`
- `Cache-Control: public, max-age=31536000`

**示例**

```bash
curl "http://localhost:26480/api/v1/video/segment?url=https://example.com/video.mp4&index=0&format=hls-fmp4" -o seg0.mp4
```

---

### 5. 获取 HLS 播放列表

生成基于 `hls-fmp4` 分片的标准 VOD M3U8 播放列表（`#EXT-X-VERSION:7`）。

```http
GET /api/v1/video/hls/playlist?url={video_url}
```

**查询参数**

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 视频源 URL（仅限 `http` / `https`） |

**响应**

- `Content-Type: application/vnd.apple.mpegurl`
- `Cache-Control: no-cache`
- 响应体中的分片 URI 指向 `/api/v1/video/segment?url=...&index=...&format=hls-fmp4`。

---

### 6. 获取 HLS TS 分片

按分片索引直接获取 `ts`（`video/MP2T`）格式分片，等价于 `format=ts` 的分片接口。

```http
GET /api/v1/video/hls/segment?url={video_url}&index={segment_index}
```

**查询参数**

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `url` | string | 是 | 视频源 URL（仅限 `http` / `https`） |
| `index` | int | 是 | 分片索引（从 `0` 开始的非负整数） |

---

## 错误响应格式

所有 `4xx` / `5xx` 错误统一返回 JSON：

```json
{
  "error": "invalid_request",
  "message": "缺少必需参数: url"
}
```

### 常见错误码

| 错误码 | HTTP 状态码 | 说明 |
|--------|-------------|------|
| `invalid_request` | `400` | 缺少必填参数、URL 协议不合法、索引无效或 `format` 不在白名单内 |
| `invalid_range` | `400` | `Range` 请求头格式错误 |
| `info_error` | `500` | `ffprobe` 获取或解析视频信息失败 |
| `transcode_error` | `500` | FFmpeg 转码失败 |
| `generate_m3u8_failed` | `500` | 生成 M3U8 播放列表失败 |
| `file_error` | `500` | 读取已缓存分片文件失败 |
| `seek_error` | `500` | 定位缓存文件偏移失败 |

---

## 客户端调用示例

### JavaScript

```javascript
const apiBase = 'http://localhost:26480/api/v1';
const videoUrl = 'https://example.com/video.mp4';

// 获取视频信息
const infoRes = await fetch(`${apiBase}/video/info?url=${encodeURIComponent(videoUrl)}`);
const info = await infoRes.json();
console.log(info.segment_count, info.segment_duration);

// 使用 hls.js 播放 M3U8
const playlistUrl = `${apiBase}/video/hls/playlist?url=${encodeURIComponent(videoUrl)}`;
```

### Python

```python
import requests

api_base = "http://localhost:26480/api/v1"
video_url = "https://example.com/video.mp4"

info = requests.get(f"{api_base}/video/info", params={"url": video_url}, timeout=30).json()
print("segments:", info["segment_count"])

seg = requests.get(
    f"{api_base}/video/segment",
    params={"url": video_url, "index": 0, "format": "hls-fmp4"},
    timeout=60,
)
seg.raise_for_status()
with open("seg0.mp4", "wb") as f:
    f.write(seg.content)
```
