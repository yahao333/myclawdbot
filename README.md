# MyClawDBot

使用 Go 语言复刻 [OpenClaw](https://github.com/openclaw/openclaw) 核心功能的 AI 编程助手。

## 项目简介

MyClawDBot 是一个 AI 编程助手，能够帮助你完成软件开发任务。它可以：
- 读写文件、搜索代码
- 执行命令、运行测试
- 分析和理解代码库
- 回答技术问题

## 核心功能

### 1. AI 对话交互
- 自然语言处理，理解用户意图
- 多轮对话上下文保持
- 代码解释和技术问答

### 2. 文件操作
- 读取和创建文件
- 代码搜索和替换
- 项目结构分析

### 3. 命令执行
- 执行 shell 命令
- 运行构建和测试
- Git 操作支持

### 4. 多渠道支持
- CLI 命令行界面
- 支持扩展到多种消息平台（Telegram、Discord、Slack 等）

### 5. 网关模式
- 本地网关运行
- 安全隔离的执行环境

## 技术栈

- **语言**: Go 1.25+
- **架构**: 模块化设计
- **API**: RESTful API

## 快速开始

```bash
# 构建项目
go build ./...

# 运行测试
go test ./...

# 运行程序
go run ./cmd/myclawdbot
```

## 项目结构

```
.
├── cmd/           # 命令行入口
├── internal/      # 内部包
├── pkg/           # 可复用包
├── api/           # API 接口
└── docs/          # 文档
```

## 参考

- 原始项目: [OpenClaw](https://github.com/openclaw/openclaw)
- 官方网站: https://openclaw.ai
- 文档: https://docs.openclaw.ai
