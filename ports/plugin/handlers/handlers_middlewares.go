// 定义中间件模块，如凭据验证

package handlers

import (
    "context" // 导入 context 包，用于传递上下文信息
    "golang.zabbix.com/plugin/ports/plugin/params" // 导入插件的参数包，用于处理参数名称
    "golang.zabbix.com/sdk/errs" // 导入 Zabbix SDK 的错误处理包，用于返回和包装错误
)

// 定义错误类型，当凭据验证失败时返回此错误
var ErrInvalidCredentials = errs.New("凭据无效") // 创建一个新的错误实例，表示凭据验证失败

// WithCredentialValidation 是一个中间件函数，用于验证传入的用户名和密码是否有效
func WithCredentialValidation(handler HandlerFunc) HandlerFunc {
    // 定义一组硬编码的有效凭据，包括用户名和密码对
    validCreds := map[string]string{
        "Zabbix": "",    // 用户名为 "Zabbix"，密码为空字符串
        "Admin":  "Foo", // 用户名为 "Admin"，密码为 "Foo"
        "User":   "Bar", // 用户名为 "User"，密码为 "Bar"
        "Test":   "Test", // 用户名为 "Test"，密码为 "Test"
    }

    // 返回一个新的处理函数，该函数会在调用原处理函数之前执行凭据验证
    return func(ctx context.Context, metricParams map[string]string) (any, error) {
        // 从传入的参数中提取用户名，并检查该用户名是否在有效凭据列表中存在
        passwd, ok := validCreds[metricParams[params.UsernameParameterName]]
        if !ok {
            // 如果用户名无效，则返回 "凭据无效" 错误
            return nil, ErrInvalidCredentials
        }

        // 如果用户名存在，继续检查传入的密码是否与预期密码匹配
        if passwd != metricParams[params.PasswordParameterName] {
            // 如果密码不匹配，则返回 "凭据无效" 错误
            return nil, ErrInvalidCredentials
        }

        // 如果用户名和密码均有效，则调用原始处理函数并返回结果
        return handler(ctx, metricParams)
    }
}
