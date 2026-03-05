// Package debug 提供调试信息收集功能
//
// 用于快速定位问题，收集和整理调试信息。
// 支持调试级别控制、结构化输出和多种信息收集方式。
//
// 使用方法:
//
//	// 基本使用
//	debug.Info("应用启动")
//
//	// 收集调试信息
//	info := debug.CollectDebugInfo(debug.Detailed, "登录失败", err)
//	fmt.Println(info.Report())
//
//	// 捕获 panic
//	defer debug.RecoverAndLog()
package debug

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"runtime"
	runtimeDebug "runtime/debug"
	"strings"
	"time"

	"github.com/yahao333/myclawdbot/internal/config"
)

// DetailLevel 调试信息详细程度
type DetailLevel int

const (
	// Basic 基础级别 - 仅关键信息
	Basic DetailLevel = iota
	// Detailed 详细级别（默认）
	Detailed
	// Comprehensive 完整级别
	Comprehensive
)

// String 返回调试级别的字符串表示
func (d DetailLevel) String() string {
	switch d {
	case Basic:
		return "basic"
	case Detailed:
		return "detailed"
	case Comprehensive:
		return "comprehensive"
	default:
		return "unknown"
	}
}

// ParseDetailLevel 解析调试级别字符串
func ParseDetailLevel(s string) DetailLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "basic", "0":
		return Basic
	case "detailed", "1":
		return Detailed
	case "comprehensive", "2":
		return Comprehensive
	default:
		return Detailed // 默认详细级别
	}
}

// Info 记录调试信息（使用默认日志记录器）
func Info(msg string) {
	log.Println("[DEBUG]", msg)
}

// Infof 记录格式化调试信息
func Infof(format string, args ...interface{}) {
	log.Printf("[DEBUG] "+format, args...)
}

// DebugEnabled 检查调试是否启用
func DebugEnabled() bool {
	switch strings.ToLower(os.Getenv("DEBUG")) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

// DebugLevel 获取调试详细级别
func DebugLevel() DetailLevel {
	level := os.Getenv("DEBUG_LEVEL")
	if level == "" {
		return Detailed
	}
	return ParseDetailLevel(level)
}

// DebugLevelFromConfig 从 config.DebugConfig 获取调试详细级别
func DebugLevelFromConfig(cfg *config.DebugConfig) DetailLevel {
	if cfg.Level == "" {
		return Detailed
	}
	return ParseDetailLevel(cfg.Level)
}

// IsDebugEnabledFromConfig 检查是否启用调试模式（从配置）
func IsDebugEnabledFromConfig(cfg *config.DebugConfig) bool {
	return cfg.Enable
}

// DebugInfo 调试信息收集器
type DebugInfo struct {
	Timestamp   string                 // 时间戳
	Level       DetailLevel            // 详细程度
	Problem     string                 // 问题描述
	Error       error                  // 错误对象
	StackTrace  string                 // 堆栈跟踪
	Environment map[string]string       // 环境变量（脱敏）
	Variables   map[string]interface{} // 关键变量
	Logs        []string               // 相关日志片段
	CodeContext []CodeLine             // 代码上下文
}

// CodeLine 代码行
type CodeLine struct {
	LineNumber int    // 行号
	Content    string // 代码内容
}

// NewDebugInfo 创建新的调试信息收集器
func NewDebugInfo(level DetailLevel, problem string, err error) *DebugInfo {
	return &DebugInfo{
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
		Level:       level,
		Problem:     problem,
		Error:       err,
		Environment: make(map[string]string),
		Variables:   make(map[string]interface{}),
		Logs:        make([]string, 0),
		CodeContext: make([]CodeLine, 0),
	}
}

// CollectDebugInfo 收集调试信息
//
// 参数：
//   - level: 详细程度
//   - problem: 问题描述
//   - err: 错误对象（可选）
//
// 返回：
//   - *DebugInfo: 调试信息收集器
func CollectDebugInfo(level DetailLevel, problem string, err error) *DebugInfo {
	info := NewDebugInfo(level, problem, err)

	// 收集堆栈跟踪
	info.StackTrace = captureStackTrace(2)

	// 收集环境变量（脱敏）
	info.collectEnvironment()

	// 完整级别收集更多信息
	if level >= Detailed {
		info.collectSystemInfo()
	}

	return info
}

// captureStackTrace 捕获堆栈跟踪
func captureStackTrace(skip int) string {
	buf := new(bytes.Buffer)
	stack := runtimeDebug.Stack()
	buf.Write(stack)
	return buf.String()
}

// collectEnvironment 收集环境变量（脱敏）
func (d *DebugInfo) collectEnvironment() {
	// 收集关键环境变量（隐藏敏感信息）
	sensitive := []string{"PASSWORD", "SECRET", "TOKEN", "API_KEY", "PRIVATE_KEY"}

	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}
		key := pair[0]
		value := pair[1]

		// 检查是否为敏感字段
		isSensitive := false
		for _, s := range sensitive {
			if strings.Contains(strings.ToUpper(key), s) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			value = "***HIDDEN***"
		}

		d.Environment[key] = value
	}
}

// collectSystemInfo 收集系统信息
func (d *DebugInfo) collectSystemInfo() {
	d.Environment["PWD"] = os.Getenv("PWD")
	d.Environment["USER"] = os.Getenv("USER")
	d.Environment["GOVERSION"] = runtime.Version()
	d.Environment["NUMCPU"] = fmt.Sprintf("%d", runtime.NumCPU())
}

// AddVariable 添加变量
func (d *DebugInfo) AddVariable(name string, value interface{}) {
	d.Variables[name] = value
}

// AddVariables 添加多个变量
func (d *DebugInfo) AddVariables(vars map[string]interface{}) {
	for k, v := range vars {
		d.Variables[k] = v
	}
}

// AddLog 添加日志片段
func (d *DebugInfo) AddLog(log string) {
	d.Logs = append(d.Logs, log)
}

// AddCodeContext 添加代码上下文
func (d *DebugInfo) AddCodeContext(file string, line int, contextLines int) {
	lines := readFileLines(file, line, contextLines)
	d.CodeContext = append(d.CodeContext, lines...)
}

// readFileLines 读取文件指定行附近的代码
func readFileLines(file string, line int, context int) []CodeLine {
	// 简化实现，实际可以读取文件内容
	return []CodeLine{
		{LineNumber: line, Content: "// (source not available)"},
	}
}

// Report 生成调试信息报告
func (d *DebugInfo) Report() string {
	var buf bytes.Buffer

	// 标题
	buf.WriteString(fmt.Sprintf("=== 调试信息报告 - %s ===\n", d.Problem))
	buf.WriteString(fmt.Sprintf("时间: %s\n", d.Timestamp))
	buf.WriteString(fmt.Sprintf("级别: %s\n\n", d.Level))

	// 错误信息
	if d.Error != nil {
		buf.WriteString(fmt.Sprintf("错误: %v\n\n", d.Error))
	}

	// 堆栈跟踪
	if d.StackTrace != "" {
		buf.WriteString("堆栈跟踪:\n")
		buf.WriteString(d.StackTrace)
		buf.WriteString("\n")
	}

	// 关键变量
	if len(d.Variables) > 0 && d.Level >= Basic {
		buf.WriteString("关键变量:\n")
		for k, v := range d.Variables {
			buf.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
		buf.WriteString("\n")
	}

	// 日志片段
	if len(d.Logs) > 0 && d.Level >= Detailed {
		buf.WriteString("相关日志:\n")
		for _, l := range d.Logs {
			buf.WriteString(fmt.Sprintf("  %s\n", l))
		}
		buf.WriteString("\n")
	}

	// 环境变量（仅在完整级别）
	if len(d.Environment) > 0 && d.Level >= Comprehensive {
		buf.WriteString("环境变量:\n")
		for k, v := range d.Environment {
			buf.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		buf.WriteString("\n")
	}

	// 代码上下文
	if len(d.CodeContext) > 0 && d.Level >= Detailed {
		buf.WriteString("代码上下文:\n")
		for _, c := range d.CodeContext {
			buf.WriteString(fmt.Sprintf("  %d: %s\n", c.LineNumber, c.Content))
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// JSON 生成 JSON 格式的调试信息
func (d *DebugInfo) JSON() string {
	// 简化实现，实际可以返回完整 JSON
	return fmt.Sprintf(`{"problem":"%s","level":"%s","timestamp":"%s"}`,
		d.Problem, d.Level, d.Timestamp)
}

// ReportMarkdown 生成 Markdown 格式的调试信息报告
func (d *DebugInfo) ReportMarkdown() string {
	var buf bytes.Buffer

	// 标题
	buf.WriteString(fmt.Sprintf("# 调试信息报告 ── %s\n\n", d.Problem))

	// 基本信息
	buf.WriteString("## 1. 问题概述\n")
	buf.WriteString(fmt.Sprintf("- 发生时间: %s\n", d.Timestamp))
	buf.WriteString(fmt.Sprintf("- 详细程度: %s\n", d.Level))

	if d.Error != nil {
		buf.WriteString(fmt.Sprintf("- 错误消息: %v\n", d.Error))
	}

	buf.WriteString("\n")

	// 错误堆栈
	if d.StackTrace != "" {
		buf.WriteString("## 2. 错误堆栈（Stack Trace）\n")
		buf.WriteString("```\n")
		buf.WriteString(d.StackTrace)
		buf.WriteString("```\n\n")
	}

	// 关键变量
	if len(d.Variables) > 0 && d.Level >= Basic {
		buf.WriteString("## 3. 关键上下文变量 / 状态\n")
		for k, v := range d.Variables {
			buf.WriteString(fmt.Sprintf("- %s: `%v`\n", k, v))
		}
		buf.WriteString("\n")
	}

	// 日志片段
	if len(d.Logs) > 0 && d.Level >= Detailed {
		buf.WriteString("## 4. 相关日志片段\n")
		buf.WriteString("```\n")
		for _, l := range d.Logs {
			buf.WriteString(l + "\n")
		}
		buf.WriteString("```\n\n")
	}

	// 环境变量（仅完整级别）
	if len(d.Environment) > 0 && d.Level >= Comprehensive {
		buf.WriteString("## 5. 环境信息\n")
		for k, v := range d.Environment {
			buf.WriteString(fmt.Sprintf("- %s: `%s`\n", k, v))
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// RecoverAndLog 捕获 panic 并记录日志
//
// 使用方法:
//
//	defer debug.RecoverAndLog()
func RecoverAndLog() {
	if r := recover(); r != nil {
		buf := new(bytes.Buffer)
		fmt.Fprintf(buf, "Panic recovered: %v\n", r)
		runtimeDebug.PrintStack()
		log.Printf("PANIC: %v\n%s", r, buf.String())
	}
}

// RecoverAndReturn 捕获 panic 并返回错误
//
// 使用方法:
//
//	defer debug.RecoverAndReturn(errPtr)
func RecoverAndReturn(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("panic: %v", r)
		log.Printf("PANIC recovered: %v", r)
		runtime.Goexit()
	}
}
