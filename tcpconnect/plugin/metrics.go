// metrics.go：定义所有 TCP 状态的监控指标，并关联具体的处理逻辑
package plugin

import (
    "context" // 用于上下文管理
    "log"     // 日志记录
    "golang.zabbix.com/plugin/tcpconnect/plugin/handlers"   // 导入 TCP 状态的处理函数包
    "golang.zabbix.com/sdk/metric" // 导入 SDK 的指标模块
    "golang.zabbix.com/sdk/plugin" // 导入插件模块
)

// 定义 TCP 连接监控项的键名
const (
    keyTcpConnectStatus = "tcp.connect.status" // TCP 连接状态的监控项键名
)

// 定义一个映射，将监控项的键与对应的处理函数关联
var metricHandlers = map[string]func(context.Context, map[string]string) (any, error){
    keyTcpConnectStatus: handlers.TcpConnectStatusHandler, // 使用正确的函数名称
}

// 定义并初始化每个监控项的指标集合
var metrics = metric.MetricSet{
    keyTcpConnectStatus: metric.New(
        "TCP 连接状态信息",
        []*metric.Param{
            metric.NewParam("status_name", "要监控的 TCP 连接状态，如 'listen' 或 'estab'"),
        },
        false,
    ),
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
