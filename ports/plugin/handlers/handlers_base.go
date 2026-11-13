// 定义基础处理函数模块
package handlers

import (
	"context"            // 导入 context 包，用于传递上下文信息
	"encoding/json"      // 导入 encoding/json 包，用于处理 JSON 编码和解码
	"fmt"                // 导入 fmt 包，用于格式化输入输出
	"net/http"           // 导入 net/http 包，处理 HTTP 客户端请求
	"os"                 // 导入 os 包，用于访问操作系统环境变量
	"golang.zabbix.com/sdk/errs" // 导入 Zabbix SDK 的错误处理包，用于包装错误
)

// 定义处理函数类型，函数接收上下文和参数，返回任意类型的数据和错误
type HandlerFunc func(
	ctx context.Context,            // 上下文信息，用于控制函数执行状态
	metricParams map[string]string, // 用于传递参数的字符串映射表
) (any, error)                      // 返回任意类型的结果和错误

// 定义 Handler 结构体，用于保存 HTTP 客户端和系统调用接口的实现
type Handler struct {
	client   *http.Client // HTTP 客户端，用于发送请求
	sysCalls systemCalls  // 系统调用接口，用于访问环境变量
}

// 定义系统调用接口，提供访问环境变量的方法
type systemCalls interface {
	environ() []string                   // 返回所有环境变量
	lookupEnv(key string) (string, bool) // 根据键名查找特定环境变量，返回值和是否找到的标志
}

// osWrapper 结构体实现 systemCalls 接口，用于实际访问操作系统环境变量
type osWrapper struct{}

// environ 方法实现 systemCalls 接口，返回当前环境中的所有环境变量
func (osWrapper) environ() []string {
	return os.Environ() // 使用 os 包中的 Environ 函数获取所有环境变量
}

// lookupEnv 方法实现 systemCalls 接口，根据指定键查找环境变量
func (osWrapper) lookupEnv(key string) (string, bool) {
	return os.LookupEnv(key) // 使用 os 包中的 LookupEnv 函数查找环境变量
}

// New 函数返回一个新的 Handler 实例，初始化 HTTP 客户端和系统调用接口
func New() *Handler {
	return &Handler{
		client:   http.DefaultClient, // 使用默认的 HTTP 客户端
		sysCalls: osWrapper{},        // 使用 osWrapper 实现的系统调用接口
	}
}

// WithJSONResponse 是一个高阶函数，包装传入的处理函数，将其结果序列化为 JSON
func WithJSONResponse(handler func(context.Context, map[string]string) (any, error)) func(context.Context, map[string]string) (any, error) {
	return func(ctx context.Context, params map[string]string) (any, error) {
		// 调用传入的处理函数并获取结果
		res, err := handler(ctx, params)
		if err != nil {
			// 如果处理函数返回错误，打印错误信息
			fmt.Printf("处理函数执行失败: %v\n", err)
			// 使用 errs 包装错误并返回
			return nil, errs.Wrap(err, "处理函数执行失败")
		}

		// 打印处理函数返回的原始结果及其类型
		fmt.Printf("处理函数返回的原始结果: %v (%T)\n", res, res)

		// 定义一个字符串变量用于保存最终输出
		var output string
		// 使用类型断言判断返回值的类型，并进行相应处理
		switch v := res.(type) {
		case int, int32, int64:
			// 如果返回值是整数类型，将其格式化为字符串
			output = fmt.Sprintf("%d", v)
			fmt.Printf("返回整数类型结果: %s\n", output)
		case float64:
			// 如果是浮点数类型，判断是否为整数值
			if v == float64(int64(v)) {
				// 如果是整数形式的浮点数，将其转为整数格式字符串
				output = fmt.Sprintf("%d", int64(v))
			} else {
				// 否则，将其格式化为保留两位小数的字符串
				output = fmt.Sprintf("%.2f", v)
			}
			fmt.Printf("返回浮点数格式结果，保留两位小数: %s\n", output)
		default:
			// 对于非数值类型的数据，尝试将其序列化为 JSON 格式
			jsonData, err := json.Marshal(res)
			if err != nil {
				// 如果序列化失败，打印错误信息并返回错误
				fmt.Printf("序列化结果为 JSON 失败: %v\n", err)
				return nil, errs.Wrap(err, "序列化结果为 JSON 失败")
			}
			// 打印序列化后的 JSON 数据
			fmt.Printf("序列化后的 JSON 数据: %s\n", jsonData)

			// 尝试将 JSON 数据反序列化为字符串，以确保不会变成 JSON 字符串格式
			if err := json.Unmarshal(jsonData, &output); err != nil {
				// 如果反序列化失败，打印错误信息并返回错误
				fmt.Printf("JSON 反序列化失败: %v\n", err)
				return nil, errs.Wrap(err, "JSON 反序列化失败")
			}
			// 打印反序列化后的数据
			fmt.Printf("反序列化后的输出数据: %v\n", output)
		}

		// 返回处理后的字符串结果
		return output, nil
	}
}
