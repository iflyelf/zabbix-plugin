// 定义基础处理函数模块
package handlers

import (
        "context"            // 导入 context 包，用于传递上下文信息
        "encoding/json"      // 导入 encoding/json 包，用于处理 JSON 编码和解码
        "fmt"                // 导入 fmt 包，用于格式化输入输出
        "golang.zabbix.com/sdk/errs" // 导入 Zabbix SDK 的错误处理包，用于包装错误
)

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

