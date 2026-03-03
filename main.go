package main

import (
	"fmt"
	"net/http"
	"time"
)

// 用户结构体
type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

// 简单的时间处理函数
func getCurrentTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// HTTP 服务器示例
func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World! 当前时间: %s", getCurrentTime())
}

// 计算函数示例
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

func main() {
	fmt.Println("=== Go 示例程序 ===")
	fmt.Printf("当前时间: %s\n\n", getCurrentTime())

	// 数组/切片示例
	numbers := []int{1, 2, 3, 4, 5}
	sum := 0
	for _, n := range numbers {
		sum += n
	}
	fmt.Printf("数组 %v 的总和: %d\n", numbers, sum)

	// 斐波那契数列示例
	fmt.Print("\n斐波那契数列前10项: ")
	for i := 0; i < 10; i++ {
		fmt.Printf("%d ", fibonacci(i))
	}
	fmt.Println()

	// Map 示例
	users := map[int]User{
		1: {ID: 1, Name: "张三", Email: "zhangsan@example.com"},
		2: {ID: 2, Name: "李四", Email: "lisi@example.com"},
	}
	fmt.Println("\n用户列表:")
	for _, user := range users {
		fmt.Printf("  ID: %d, Name: %s, Email: %s\n", user.ID, user.Name, user.Email)
	}

	// 启动 HTTP 服务器
	fmt.Println("\n启动 HTTP 服务器在 http://localhost:8080")
	http.HandleFunc("/", helloHandler)
	http.ListenAndServe(":8080", nil)
}
