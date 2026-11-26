.PHONY: build run test clean install-deps fmt lint docker-build docker-run help

BINARY_NAME := plugstream
BUILD_DIR := build
CMD_DIR := cmd/server
CONFIG_FILE := config.yaml
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := help

## help: 显示帮助信息
help:
	@echo "PlugStream - 实时视频转码流式传输服务"
	@echo ""
	@echo "可用命令:"
	@echo "  make build        - 编译项目"
	@echo "  make run          - 运行服务"
	@echo "  make test         - 运行测试"
	@echo "  make clean        - 清理构建文件"
	@echo "  make install-deps - 安装并整理依赖"
	@echo "  make fmt          - 格式化代码"
	@echo "  make lint         - 运行代码检查"
	@echo "  make docker-build - 构建 Docker 镜像"
	@echo "  make docker-run   - 运行 Docker 容器"
	@echo "  make help         - 显示此帮助信息"

## build: 编译项目
build:
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "编译完成: $(BUILD_DIR)/$(BINARY_NAME) ($(VERSION))"

## run: 运行服务
run:
	@go run ./$(CMD_DIR) -config=$(CONFIG_FILE)

## test: 运行测试
test:
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "测试完成，覆盖率报告: coverage.html"

## clean: 清理构建文件
clean:
	@rm -rf $(BUILD_DIR)
	@rm -f PlugStream plugstream coverage.out coverage.html
	@echo "清理完成"

## install-deps: 安装并整理依赖
install-deps:
	@go mod download
	@go mod tidy

## fmt: 格式化代码
fmt:
	@go fmt ./...

## lint: 代码检查
lint:
	@golangci-lint run ./...

## docker-build: 构建 Docker 镜像
docker-build:
	@docker build --build-arg VERSION=$(VERSION) -t plugstream:latest .

## docker-run: 运行 Docker 容器
docker-run:
	@docker run -p 26480:26480 -v $(PWD)/config.yaml:/app/config.yaml:ro plugstream:latest
