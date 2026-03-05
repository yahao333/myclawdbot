package browser

import "os"

// WriteFile 写入文件
// path: 文件路径
// data: 文件内容
func WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
