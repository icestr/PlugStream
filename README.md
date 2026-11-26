# PlugStream

English | [简体中文](README_ZH.md)

[![CI](https://github.com/icestr/PlugStream/actions/workflows/ci.yml/badge.svg)](https://github.com/icestr/PlugStream/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/icestr/PlugStream)](https://github.com/icestr/PlugStream/releases)

A lightweight real-time video transcoding and streaming proxy built with Go and FFmpeg. PlugStream transcodes remote HTTP/HTTPS video sources on demand into H.264/AAC segments, caches them locally, and serves them over HTTP Range or HLS (fMP4 / MPEG-TS).

## Features

- **On-demand segmented transcoding**: Splits remote videos into fixed-duration segments (10s by default) and streams output while FFmpeg is still writing.
- **HLS and direct MP4 streaming**: Generates standard VOD M3U8 playlists (`#EXT-X-VERSION:7`) and serves `mp4` (fMP4), `hls-fmp4`, and `ts` segments.
- **HTTP Range support**: Handles `bytes=start-end`, `bytes=start-`, and `bytes=-suffix` requests for progressive playback and resumption.
- **Deduplicated segment cache**: Uses MD5-sharded local disk caching with per-segment locks and periodic expiration cleanup.
- **Concurrency control**: Caps active FFmpeg jobs with a semaphore to prevent CPU exhaustion.

## Quick Start

### Prerequisites

- Go 1.24 or later (for building from source)
- `ffmpeg` and `ffprobe` available in `PATH` (or configured via `ffmpeg.binary_path` in `config.yaml`)

### Option 1: Run from Source

```bash
git clone https://github.com/icestr/PlugStream.git
cd PlugStream
make install-deps
make run
```

### Option 2: Docker Compose

```bash
git clone https://github.com/icestr/PlugStream.git
cd PlugStream
docker compose up -d
```

### Option 3: Build Binary

```bash
make build
./build/plugstream -config config.yaml
```

Verify the server is running:

```bash
curl http://localhost:26480/api/v1/health
```

## HTTP API

Base URL: `http://localhost:26480/api/v1`

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Service health and version check |
| `GET` | `/video/info?url=<video_url>` | Probe metadata and total segment count |
| `GET` | `/video/stream?url=<video_url>` | Stream fMP4 output with `Range` support |
| `GET` | `/video/segment?url=<video_url>&index=<n>&format=<mp4\|ts\|hls-fmp4>` | Fetch a specific transcoded segment |
| `GET` | `/video/hls/playlist?url=<video_url>` | Generate HLS M3U8 playlist (`hls-fmp4`) |
| `GET` | `/video/hls/segment?url=<video_url>&index=<n>` | Fetch an HLS MPEG-TS segment |

Example:

```bash
# Fetch HLS playlist
curl "http://localhost:26480/api/v1/video/hls/playlist?url=https://example.com/video.mp4"

# Fetch segment 0 as fMP4
curl "http://localhost:26480/api/v1/video/segment?url=https://example.com/video.mp4&index=0&format=hls-fmp4" -o seg0.mp4
```

An interactive browser demo is available in [`examples/player.html`](examples/player.html).

## Configuration

Settings live in [`config.yaml`](config.yaml):

- `server`: Listen host/port, request timeout (seconds), and `max_concurrent_tasks`.
- `ffmpeg`: Optional `binary_path`, `video_codec`, `audio_codec`, bitrates, `preset`, and `crf`.
- `segment`: `duration` (seconds) and `format` (`fmp4`).
- `cache`: Cache `directory`, `expiration` (hours), `max_size` (GB), and `cleanup_interval` (hours).
- `log`: `level` (`debug`/`info`/`warn`/`error`), `format` (`json`/`text`), and optional `output` file path.

## Documentation

- [Usage Guide](docs/USAGE.md)
- [API Reference](docs/API.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [Changelog](CHANGELOG.md)

## Security Notice

PlugStream forwards user-supplied `http`/`https` URLs to `ffmpeg` and `ffprobe`. Do not expose an unauthenticated instance directly to untrusted public traffic without placing it behind a gateway that enforces authentication, rate limiting, and allowed source domains. See [SECURITY.md](SECURITY.md) for details.

## License

Licensed under the [MIT License](LICENSE).
