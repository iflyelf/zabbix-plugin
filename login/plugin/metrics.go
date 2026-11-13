// metrics.go 定义所有登录监控指标及其参数设置
package plugin

import (
    "context"                                 // 用于管理插件中的上下文传递，便于控制执行流程和取消操作
    "log"                                     // 用于在控制台或日志文件中记录日志信息
    "golang.zabbix.com/plugin/login/plugin/handlers" // 引入登录失败处理程序库，处理不同的监控项
    "golang.zabbix.com/sdk/metric"            // 引入 SDK 的指标模块，定义和管理监控指标
    "golang.zabbix.com/sdk/plugin"            // 引入插件管理模块，用于插件注册和初始化
)

// 定义登录监控项的键名，用于标识每个监控项
const (
    keyLoginFailedAttempts = "login.failed" // 监控登录失败次数
)

// 定义一个映射，将每个监控项的键与对应的处理函数关联，以便动态调用
var metricHandlers = map[string]func(context.Context, map[string]string) (any, error){
    keyLoginFailedAttempts: handlers.WithJSONResponse(handlers.LoginFailedAttemptsHandler),  // 登录失败次数处理函数
}

// 初始化每个监控项的指标集合，定义指标的名称、参数和是否聚合
var metrics = metric.MetricSet{
    keyLoginFailedAttempts: metric.New("失败登录尝试", nil, false),  // 登录失败尝试指标，不需要参数
}

// init 函数在程序启动时自动运行，将所有定义的监控项注册到插件中
func init() {
    // 调用插件 SDK 的 RegisterMetrics 函数，将监控项集合注册到插件
    err := plugin.RegisterMetrics(&Impl, Name, metrics.List()...)
    if err != nil { // 检查注册过程中是否有错误
        log.Fatalf("Failed to register metrics: %v", err) // 如果有错误，记录并终止程序
    }
}

// getHandlerFunc 根据给定的监控项键名返回相应的处理函数
func getHandlerFunc(key string) func(context.Context, map[string]string) (any, error) {
    return metricHandlers[key] // 从映射中查找并返回对应的处理函数
}
