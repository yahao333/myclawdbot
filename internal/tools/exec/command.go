package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// CommandTool 命令执行工具
type CommandTool struct {
	allowedCommands []string
	maxExecTime    time.Duration
	dir            string
}

func NewCommandTool(allowedCommands []string, maxExecTime int) *CommandTool {
	if maxExecTime <= 0 {
		maxExecTime = 300 // 默认 5 分钟
	}

	// 默认允许的命令
	if len(allowedCommands) == 0 {
		allowedCommands = []string{"go", "git", "ls", "cat", "pwd", "echo", "mkdir", "rm", "cp", "mv", "cd", "find", "grep"}
	}

	return &CommandTool{
		allowedCommands: allowedCommands,
		maxExecTime:    time.Duration(maxExecTime) * time.Second,
	}
}

func (t *CommandTool) Name() string        { return "bash" }
func (t *CommandTool) Description() string { return "执行 shell 命令" }
func (t *CommandTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "要执行的命令",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "超时时间（秒）",
				"default":     60,
			},
			"description": map[string]any{
				"type":        "string",
				"description": "命令描述",
			},
		},
		"required": []string{"command"},
	}
}

func (t *CommandTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("command is required")
	}

	// 安全检查：检查命令是否允许
	if !t.isCommandAllowed(command) {
		return "", fmt.Errorf("command not allowed: %s", extractCommand(command))
	}

	// 超时设置
	timeout := 60
	if timeoutVal, ok := params["timeout"].(float64); ok {
		timeout = int(timeoutVal)
	}
	if timeout > int(t.maxExecTime.Seconds()) {
		timeout = int(t.maxExecTime.Seconds())
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// 执行命令
	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	// 设置工作目录
	if t.dir != "" {
		cmd.Dir = t.dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.String() != "" {
		output += "\nSTDERR: " + stderr.String()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %d seconds", timeout)
		}
		return output + "\n" + fmt.Sprintf("error: %v", err), nil
	}

	return output, nil
}

func (t *CommandTool) isCommandAllowed(command string) bool {
	cmd := extractCommand(command)
	for _, allowed := range t.allowedCommands {
		if cmd == allowed {
			return true
		}
	}
	return false
}

func extractCommand(command string) string {
	// 去除前导空白
	i := 0
	for ; i < len(command); i++ {
		if command[i] != ' ' && command[i] != '\t' && command[i] != '\n' && command[i] != '\r' {
			break
		}
	}
	command = command[i:]

	if len(command) == 0 {
		return ""
	}

	// 处理引号包围的命令
	if command[0] == '"' || command[0] == '\'' {
		quoteChar := command[0]
		// 找到结束引号
		end := 1
		for ; end < len(command); end++ {
			if command[end] == quoteChar {
				// 检查引号后面是否还有字符（可能有空格分隔）
				break
			}
		}
		if end > 1 {
			return command[1:end]
		}
	}

	// 提取第一个单词
	wordEnd := 0
	for ; wordEnd < len(command); wordEnd++ {
		if command[wordEnd] == ' ' || command[wordEnd] == '\t' ||
			command[wordEnd] == '\n' || command[wordEnd] == '\r' {
			break
		}
	}

	if wordEnd > 0 {
		return command[:wordEnd]
	}

	return command
}

// Bash 别名
type BashTool = CommandTool

// NewBashTool 创建 bash 工具
func NewBashTool(allowedCommands []string, maxExecTime int) *BashTool {
	return NewCommandTool(allowedCommands, maxExecTime)
}
