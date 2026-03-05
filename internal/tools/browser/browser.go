// Package browser 浏览器自动化工具包
// 提供浏览器控制功能，包括截图、点击、输入等操作
// 使用 chromedp 库实现，支持 Chrome/Chromium 自动化
package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/yahao333/myclawdbot/internal/tools"
)

// BrowserTool 浏览器控制工具
// 实现 Tool 接口，提供浏览器自动化能力
type BrowserTool struct {
	headless    bool           // 是否无头模式
	timeout     time.Duration  // 操作超时时间
	screenshotPath string     // 截图保存路径
}

// NewBrowserTool 创建浏览器工具实例
// headless: 是否使用无头模式（不显示浏览器窗口）
func NewBrowserTool(headless bool) *BrowserTool {
	return &BrowserTool{
		headless:    headless,
		timeout:     30 * time.Second,
		screenshotPath: "/tmp",
	}
}

// NewBrowserToolWithTimeout 创建带超时配置的浏览器工具
func NewBrowserToolWithTimeout(headless bool, timeout time.Duration) *BrowserTool {
	return &BrowserTool{
		headless:    headless,
		timeout:     timeout,
		screenshotPath: "/tmp",
	}
}

// Name 返回工具名称
func (t *BrowserTool) Name() string {
	return "browser"
}

// Description 返回工具描述
func (t *BrowserTool) Description() string {
	return "控制浏览器：截图、点击元素、填写表单、导航到 URL。支持无头模式。"
}

// Parameters 返回工具参数 schema
func (t *BrowserTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "操作类型: screenshot, click, type, navigate, scroll, evaluate",
				"enum":        []string{"screenshot", "click", "type", "navigate", "scroll", "evaluate"},
			},
			"url": map[string]any{
				"type":        "string",
				"description": "要导航到的 URL（navigate 操作需要）",
			},
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS 选择器（click, type, scroll 操作需要）",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "要输入的文本（type 操作需要）",
			},
			"script": map[string]any{
				"type":        "string",
				"description": "要执行的 JavaScript 代码（evaluate 操作需要）",
			},
			"filename": map[string]any{
				"type":        "string",
				"description": "截图保存的文件名（screenshot 操作需要）",
			},
		},
		"required": []string{"action"},
	}
}

// Execute 执行浏览器操作
// params 包含操作参数：
//   - action: 操作类型 (screenshot, click, type, navigate, scroll, evaluate)
//   - url: URL (navigate)
//   - selector: CSS 选择器 (click, type, scroll)
//   - text: 文本 (type)
//   - script: JavaScript (evaluate)
//   - filename: 文件名 (screenshot)
func (t *BrowserTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return "", fmt.Errorf("missing action parameter")
	}

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// 分配浏览器
	allocCtx, cancel := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", t.headless),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
		)...,
	)
	defer cancel()

	// 创建浏览器 context
	browserCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// 根据操作类型执行
	switch action {
	case "screenshot":
		return t.doScreenshot(browserCtx, params)
	case "navigate":
		return t.doNavigate(browserCtx, params)
	case "click":
		return t.doClick(browserCtx, params)
	case "type":
		return t.doType(browserCtx, params)
	case "scroll":
		return t.doScroll(browserCtx, params)
	case "evaluate":
		return t.doEvaluate(browserCtx, params)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

// doScreenshot 截图操作
func (t *BrowserTool) doScreenshot(ctx context.Context, params map[string]any) (string, error) {
	filename, _ := params["filename"].(string)
	if filename == "" {
		filename = fmt.Sprintf("browser_%d.png", time.Now().Unix())
	}

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.FullScreenshot(&buf, 100),
	); err != nil {
		return "", fmt.Errorf("screenshot failed: %w", err)
	}

	// 保存到文件
	path := t.screenshotPath + "/" + filename
	if err := saveFile(path, buf); err != nil {
		return "", fmt.Errorf("save screenshot failed: %w", err)
	}

	return fmt.Sprintf("Screenshot saved to: %s", path), nil
}

// doNavigate 导航操作
func (t *BrowserTool) doNavigate(ctx context.Context, params map[string]any) (string, error) {
	url, _ := params["url"].(string)
	if url == "" {
		return "", fmt.Errorf("missing url parameter")
	}

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByJSPath),
	); err != nil {
		return "", fmt.Errorf("navigate failed: %w", err)
	}

	// 获取页面标题
	var title string
	if err := chromedp.Run(ctx,
		chromedp.Title(&title),
	); err != nil {
		return "", fmt.Errorf("get title failed: %w", err)
	}

	return fmt.Sprintf("Navigated to: %s\nPage title: %s", url, title), nil
}

// doClick 点击操作
func (t *BrowserTool) doClick(ctx context.Context, params map[string]any) (string, error) {
	selector, _ := params["selector"].(string)
	if selector == "" {
		return "", fmt.Errorf("missing selector parameter")
	}

	if err := chromedp.Run(ctx,
		chromedp.Click(selector, chromedp.ByQuery),
	); err != nil {
		return "", fmt.Errorf("click failed: %w", err)
	}

	return fmt.Sprintf("Clicked element: %s", selector), nil
}

// doType 输入文本操作
func (t *BrowserTool) doType(ctx context.Context, params map[string]any) (string, error) {
	selector, _ := params["selector"].(string)
	text, _ := params["text"].(string)

	if selector == "" {
		return "", fmt.Errorf("missing selector parameter")
	}
	if text == "" {
		return "", fmt.Errorf("missing text parameter")
	}

	if err := chromedp.Run(ctx,
		chromedp.SetValue(selector, text, chromedp.ByQuery),
	); err != nil {
		return "", fmt.Errorf("type failed: %w", err)
	}

	return fmt.Sprintf("Typed text into: %s", selector), nil
}

// doScroll 滚动操作
func (t *BrowserTool) doScroll(ctx context.Context, params map[string]any) (string, error) {
	selector, _ := params["selector"].(string)

	var script string
	if selector != "" {
		script = fmt.Sprintf(`
			document.querySelector('%s').scrollIntoView();
		`, selector)
	} else {
		script = `window.scrollTo(0, document.body.scrollHeight);`
	}

	if err := chromedp.Run(ctx,
		chromedp.Evaluate(script, nil),
	); err != nil {
		return "", fmt.Errorf("scroll failed: %w", err)
	}

	return "Scrolled page", nil
}

// doEvaluate 执行 JavaScript
func (t *BrowserTool) doEvaluate(ctx context.Context, params map[string]any) (string, error) {
	script, _ := params["script"].(string)
	if script == "" {
		return "", fmt.Errorf("missing script parameter")
	}

	var result string
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(script, &result),
	); err != nil {
		return "", fmt.Errorf("evaluate failed: %w", err)
	}

	if result == "" {
		return "Script executed successfully", nil
	}
	return fmt.Sprintf("Result: %s", result), nil
}

// saveFile 保存文件
func saveFile(path string, data []byte) error {
	return WriteFile(path, data)
}

// 确保实现 Tool 接口
var _ tools.Tool = (*BrowserTool)(nil)
