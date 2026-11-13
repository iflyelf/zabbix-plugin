// handlers_http_status.go：定义 HTTP 状态检查的处理逻辑
package handlers

import (
	"context"       // 引入上下文包，用于控制请求的超时和取消
	"fmt"           // 引入格式化包，用于生成错误信息
	"net/http"      // 引入 HTTP 客户端包，用于发送 HTTP 请求
	"time"          // 引入时间包，用于设置请求超时时间
)

// CheckHttpStatus 使用 HTTP GET 请求检查指定端口的连通性
// 参数:
//   - ctx: 上下文参数，用于控制请求的超时和取消
//   - metricParams: 用于传入参数（包括端口号）
// 返回值:
//   - int: 1 表示端口正常且有 Header 数据，0 表示端口异常或无 Header 数据
//   - error: 当端口号未指定或请求失败时返回错误
func CheckHttpStatus(ctx context.Context, metricParams map[string]string) (any, error) {
	// 从参数中提取端口号
	port, exists := metricParams["Port"]
	if !exists || port == "" {
		return 0, fmt.Errorf("端口号未指定") // 若未指定端口号，返回错误信息
	}

	// 构建请求 URL，使用本地地址 127.0.0.1 和指定端口
	url := "http://127.0.0.1:" + port

	// 创建一个 HTTP 客户端实例，并设置超时时间为 3 秒
	client := http.Client{
		Timeout: 3 * time.Second, // 设置请求的最大等待时间
	}

	// 创建带有上下文的 GET 请求，便于控制超时或取消
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("创建 HTTP 请求失败: %v", err) // 若请求创建失败，返回错误信息
	}

	// 发送请求并接收响应
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil // 请求失败返回 0，表示端口不可访问或响应失败
	}
	defer resp.Body.Close() // 确保函数退出时关闭响应体

	// 检查响应的 Header 是否存在数据
	if len(resp.Header) > 0 {
		return 1, nil // 若 Header 存在数据，返回 1 表示端口正常
	}

	// 若 Header 为空，返回 0 表示端口无响应或异常
	return 0, nil
}

