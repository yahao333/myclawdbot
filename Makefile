# Go 项目变量
BINARY_NAME=myclawdbot
GO_CMD=go
GO_FILES=$(shell $(GO_CMD) list -m -f '{{.Dir}}')
MAIN_FILE=cmd/myclawdbot/main.go

# 构建变量
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"

# 颜色输出
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[0;33m
NC=\033[0m

.PHONY: all build clean test vet fmt lint install run help

# 默认目标
all: help

# 显示帮助信息
help:
	@echo "${GREEN}MyClawDBot Makefile${NC}"
	@echo ""
	@echo "可用目标:"
	@echo "  build     - 构建二进制文件"
	@echo "  clean     - 清理构建产物"
	@echo "  test      - 运行测试"
	@echo "  vet       - 运行 go vet 检查"
	@echo "  fmt       - 格式化代码"
	@echo "  lint      - 运行代码检查 (需要 golangci-lint)"
	@echo "  install   - 安装依赖"
	@echo "  run       - 运行程序"
	@echo "  help      - 显示帮助信息"

# 构建二进制文件
build:
	@echo "${YELLOW}Building ${BINARY_NAME}...${NC}"
	@mkdir -p bin
	$(GO_CMD) build $(LDFLAGS) -o bin/$(BINARY_NAME) $(MAIN_FILE)
	@echo "${GREEN}Build complete: bin/$(BINARY_NAME)${NC}"

# 清理构建产物
clean:
	@echo "${YELLOW}Cleaning build artifacts...${NC}"
	rm -rf bin/
	$(GO_CMD) clean
	@echo "${GREEN}Clean complete${NC}"

# 运行测试
test:
	@echo "${YELLOW}Running tests...${NC}"
	$(GO_CMD) test -v -race -coverprofile=coverage.html -covermode=atomic ./...

# 运行 go vet 检查
vet:
	@echo "${YELLOW}Running go vet...${NC}"
	$(GO_CMD) vet ./...

# 格式化代码
fmt:
	@echo "${YELLOW}Formatting code...${NC}"
	$(GO_CMD) fmt ./...
	@echo "${GREEN}Format complete${NC}"

# 运行代码检查
lint:
	@echo "${YELLOW}Running linter...${NC}"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "${RED}golangci-lint not found, skipping...${NC}"; \
	fi

# 安装依赖
install:
	@echo "${YELLOW}Installing dependencies...${NC}"
	$(GO_CMD) mod download
	$(GO_CMD) mod tidy
	@echo "${GREEN}Dependencies installed${NC}"

# 运行程序
run: build
	@echo "${YELLOW}Running ${BINARY_NAME}...${NC}"
	./bin/$(BINARY_NAME)
