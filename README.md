# MyClawDBot

一个强大的 AI 编程助手，帮助你完成软件开发任务。

## 项目简介

MyClawDBot 是一个 AI 编程助手，能够通过自然语言帮助你完成各种软件开发任务。它可以：
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
│   ├── channel/   # 消息渠道
│   ├── config/    # 配置管理
│   ├── llm/       # LLM 客户端
│   ├── session/   # 会话管理
│   └── tools/     # 工具集
│       ├── exec/  # 命令执行
│       ├── file/ # 文件操作
│       └── web/  # 网络请求
└── pkg/           # 公共包
```

## 配置

可通过环境变量配置：

| 变量 | 说明 |
|-----|------|
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 |
| `OPENAI_API_KEY` | OpenAI API 密钥 |
| `MINIMAX_API_KEY` | MiniMax API 密钥 |
| `SESSION_STORAGE_DIR` | 会话存储目录 |

## 参考

- 官方网站: https://openclaw.ai
- 文档: https://docs.openclaw.ai
