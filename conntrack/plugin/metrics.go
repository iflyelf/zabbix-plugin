// metrics.go：定义所有 conntrack 状态的监控指标，并关联具体的处理逻辑
package plugin

import (
    "context" // 用于上下文管理
    "log"     // 用于日志记录
    "golang.zabbix.com/plugin/conntrack/plugin/handlers" // 导入 conntrack 状态的处理函数包
    "golang.zabbix.com/sdk/metric" // 导入 SDK 的指标模块
    "golang.zabbix.com/sdk/plugin" // 导入插件模块
)

// 定义 conntrack 连接监控项的键名
const (
    keyConntrackInfo   = "conntrack.info"   // conntrack 的信息状态键
    keyConntrackNumber = "conntrack.number" // conntrack 的连接数状态键
)

// 定义一个映射，将每个监控项的键与对应的处理函数关联
var metricHandlers = map[string]func(context.Context, map[string]string) (any, error){
    keyConntrackInfo:   handlers.ConntrackInfoHandler,   // conntrack info 处理函数
    keyConntrackNumber: handlers.ConntrackNumberHandler, // conntrack number 处理函数
}

// 定义并初始化每个监控项的指标集合
var metrics = metric.MetricSet{
    keyConntrackInfo:   metric.New("获取 conntrack 状态信息", nil, false),
    keyConntrackNumber: metric.New("获取当前 conntrack 连接数", nil, false),
}

// init 函数在程序启动时运行，将所有监控项注册到插件中
func init() {
    err := plugin.RegisterMetrics(&Impl, Name, metrics.List()...)
    if err != nil {
        log.Fatalf("注册指标集合失败: %v", err)
    }
}

// getHandlerFunc 获取与给定键相关的处理函数
func getHandlerFunc(key string) func(context.Context, map[string]string) (any, error) {
    // 根据键名返回具体处理函数
    return metricHandlers[key]
}
