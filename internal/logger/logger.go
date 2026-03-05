// Package logger 提供统一的日志记录功能
//
// 支持结构化日志、日志级别控制、调试模式和多种输出方式。
// 使用方法:
//
//	logger := logger.New()
//	logger.Info("应用启动", logger.String("version", "1.0.0"))
//	logger.Debug("调试信息", logger.Int("count", 42))
//	logger.Error("发生错误", logger.Err(err))
package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	// LevelDebug 调试级别
	LevelDebug Level = iota
	// LevelInfo 信息级别
	LevelInfo
	// LevelWarn 警告级别
	LevelWarn
	// LevelError 错误级别
	LevelError
	// LevelSilent 静默模式，不输出任何日志
	LevelSilent
)

// String 返回日志级别的字符串表示
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel 解析日志级别字符串
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "0":
		return LevelDebug
	case "info", "1":
		return LevelInfo
	case "warn", "warning", "2":
		return LevelWarn
	case "error", "3":
		return LevelError
	case "silent", "4":
		return LevelSilent
	default:
		return LevelInfo // 默认 info 级别
	}
}

// Logger 日志记录器
//
// 提供结构化日志记录功能，支持多种日志级别和输出方式。
type Logger struct {
	mu         sync.Mutex
	output     io.Writer
	level      Level
	prefix     string
	flags      int
	jsonFormat bool
	timeFormat string
}

// Option 配置 Logger 的选项函数
type Option func(*Logger)

// WithOutput 设置日志输出目标（可以是文件路径或 io.Writer）
func WithOutput(w any) Option {
	return func(l *Logger) {
		switch v := w.(type) {
		case io.Writer:
			l.output = v
		case string:
			if v != "" {
				if f, err := os.OpenFile(v, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
					l.output = f
				}
			}
		}
	}
}

// WithLevel 设置日志级别
func WithLevel(level Level) Option {
	return func(l *Logger) {
		l.level = level
	}
}

// WithPrefix 设置日志前缀
func WithPrefix(prefix string) Option {
	return func(l *Logger) {
		l.prefix = prefix
	}
}

// WithJSONFormat 设置是否使用 JSON 格式
func WithJSONFormat(enabled bool) Option {
	return func(l *Logger) {
		l.jsonFormat = enabled
	}
}

// WithTimeFormat 设置时间格式
func WithTimeFormat(format string) Option {
	return func(l *Logger) {
		l.timeFormat = format
	}
}

// WithEnv 从环境变量加载配置
//
// 支持以下环境变量：
//   - LOG_LEVEL: 日志级别 (debug, info, warn, error)
//   - LOG_JSON: 是否使用 JSON 格式 (true/false)
//   - LOG_OUTPUT: 输出文件路径 (空则输出到 stdout)
func WithEnv() Option {
	return func(l *Logger) {
		if level := os.Getenv("LOG_LEVEL"); level != "" {
			l.level = ParseLevel(level)
		}
		if jsonStr := os.Getenv("LOG_JSON"); jsonStr != "" {
			l.jsonFormat = strings.ToLower(jsonStr) == "true"
		}
		if output := os.Getenv("LOG_OUTPUT"); output != "" {
			if f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				l.output = f
			}
		}
	}
}

// 默认 Logger
var defaultLogger *Logger
var defaultOnce sync.Once

// Default 获取默认日志记录器
//
// 默认配置：
//   - 输出：标准输出
//   - 级别：Info
//   - 格式：文本
func Default() *Logger {
	defaultOnce.Do(func() {
		defaultLogger = New()
	})
	return defaultLogger
}

// New 创建新的日志记录器
func New(opts ...Option) *Logger {
	l := &Logger{
		output:     os.Stdout,
		level:      LevelInfo,
		prefix:     "",
		flags:      log.LstdFlags,
		jsonFormat: false,
		timeFormat: time.RFC3339,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// NewFromConfig 从配置创建日志记录器
func NewFromConfig(cfg *Config) *Logger {
	opts := []Option{
		WithLevel(cfg.Level),
		WithJSONFormat(cfg.JSONFormat),
	}

	if cfg.Output != "" {
		opts = append(opts, WithOutput(cfg.Output))
	}

	if cfg.Prefix != "" {
		opts = append(opts, WithPrefix(cfg.Prefix))
	}

	return New(opts...)
}

// Config 日志配置
type Config struct {
	Level      Level
	JSONFormat bool
	Output     string
	Prefix     string
}

// logEntry 日志条目
type logEntry struct {
	Time    string                 `json:"time"`
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Caller  string                 `json:"caller,omitempty"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// Log 记录日志（内部使用）
func (l *Logger) Log(level Level, msg string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := logEntry{
		Time:    time.Now().Format(l.timeFormat),
		Level:   level.String(),
		Message: msg,
		Fields:  fields,
	}

	// 获取调用者信息
	if l.flags&(log.Lshortfile|log.Llongfile) != 0 {
		if _, file, line, ok := runtime.Caller(2); ok {
			entry.Caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
		}
	}

	if l.jsonFormat {
		data, err := json.Marshal(entry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal log entry: %v\n", err)
			return
		}
		fmt.Fprintln(l.output, string(data))
	} else {
		// 文本格式
		parts := []string{
			fmt.Sprintf("[%s]", entry.Time),
			fmt.Sprintf("[%s]", entry.Level),
		}
		if entry.Caller != "" {
			parts = append(parts, fmt.Sprintf("(%s)", entry.Caller))
		}
		parts = append(parts, entry.Message)

		if len(entry.Fields) > 0 {
			fieldStrs := make([]string, 0, len(entry.Fields))
			for k, v := range entry.Fields {
				fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%v", k, v))
			}
			parts = append(parts, strings.Join(fieldStrs, " "))
		}

		fmt.Fprintln(l.output, strings.Join(parts, " "))
	}
}

// Debug 记录调试级别日志
func (l *Logger) Debug(msg string, fields ...Field) {
	l.Log(LevelDebug, msg, fieldsToMap(fields))
}

// Debugf 记录调试级别日志（格式化）
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Log(LevelDebug, fmt.Sprintf(format, args...), nil)
}

// Info 记录信息级别日志
func (l *Logger) Info(msg string, fields ...Field) {
	l.Log(LevelInfo, msg, fieldsToMap(fields))
}

// Infof 记录信息级别日志（格式化）
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Log(LevelInfo, fmt.Sprintf(format, args...), nil)
}

// Warn 记录警告级别日志
func (l *Logger) Warn(msg string, fields ...Field) {
	l.Log(LevelWarn, msg, fieldsToMap(fields))
}

// Warnf 记录警告级别日志（格式化）
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Log(LevelWarn, fmt.Sprintf(format, args...), nil)
}

// Error 记录错误级别日志
func (l *Logger) Error(msg string, fields ...Field) {
	l.Log(LevelError, msg, fieldsToMap(fields))
}

// Errorf 记录错误级别日志（格式化）
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Log(LevelError, fmt.Sprintf(format, args...), nil)
}

// Fatal 记录致命错误并退出
func (l *Logger) Fatal(msg string, fields ...Field) {
	l.Log(LevelError, msg, fieldsToMap(fields))
	os.Exit(1)
}

// Fatalf 记录致命错误并退出（格式化）
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Log(LevelError, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

// With 返回带有额外字段的日志记录器
func (l *Logger) With(fields ...Field) *Logger {
	return &Logger{
		output:     l.output,
		level:      l.level,
		prefix:     l.prefix,
		flags:      l.flags,
		jsonFormat: l.jsonFormat,
		timeFormat: l.timeFormat,
	}
}

// Field 日志字段
type Field struct {
	Key   string
	Value interface{}
}

// fieldsToMap 将字段切片转换为 map
func fieldsToMap(fields []Field) map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}
	m := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	return m
}

// String 创建字符串字段
func String(key string, value string) Field {
	return Field{Key: key, Value: value}
}

// Int 创建整数字段
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 创建 64 位整数字段
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 创建浮点数字段
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool 创建布尔字段
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Err 创建错误字段
func Err(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

// Any 创建任意类型字段
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Strings 创建字符串数组字段
func Strings(key string, value []string) Field {
	return Field{Key: key, Value: value}
}

// 全局便捷函数

var _default *Logger

func init() {
	_default = Default()
}

// SetDefault 设置默认日志记录器
func SetDefault(l *Logger) {
	_default = l
}

// Debug 记录调试级别日志（使用默认记录器）
func Debug(msg string, fields ...Field) {
	_default.Debug(msg, fields...)
}

// Debugf 记录调试级别日志（格式化）
func Debugf(format string, args ...interface{}) {
	_default.Debugf(format, args...)
}

// Info 记录信息级别日志（使用默认记录器）
func Info(msg string, fields ...Field) {
	_default.Info(msg, fields...)
}

// Infof 记录信息级别日志（格式化）
func Infof(format string, args ...interface{}) {
	_default.Infof(format, args...)
}

// Warn 记录警告级别日志（使用默认记录器）
func Warn(msg string, fields ...Field) {
	_default.Warn(msg, fields...)
}

// Warnf 记录警告级别日志（格式化）
func Warnf(format string, args ...interface{}) {
	_default.Warnf(format, args...)
}

// Error 记录错误级别日志（使用默认记录器）
func Error(msg string, fields ...Field) {
	_default.Error(msg, fields...)
}

// Errorf 记录错误级别日志（格式化）
func Errorf(format string, args ...interface{}) {
	_default.Errorf(format, args...)
}

// Fatal 记录致命错误并退出
func Fatal(msg string, fields ...Field) {
	_default.Fatal(msg, fields...)
}

// Fatalf 记录致命错误并退出（格式化）
func Fatalf(format string, args ...interface{}) {
	_default.Fatalf(format, args...)
}
