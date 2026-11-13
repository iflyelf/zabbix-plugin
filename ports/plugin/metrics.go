// metrics.go：定义端口插件的所有监控指标及其对应的处理函数
package plugin

import (
	"context" // 上下文管理
	"log"     // 日志管理

	"golang.zabbix.com/plugin/ports/plugin/handlers" // 引入 handlers 包，包含各个监控项的处理函数
	"golang.zabbix.com/sdk/metric"                   // 引入 SDK 的指标模块
	"golang.zabbix.com/sdk/plugin"                   // 引入 SDK 的插件模块
)

// 定义端口监控项的键名，用于标识每个监控项
const (
	keyTcpPortDiscovery  = "ports.tcp.discovery"  // TCP 端口发现
	keyUdpPortDiscovery  = "ports.udp.discovery"  // UDP 端口发现
	keyHttpPortDiscovery = "ports.http.discovery" // HTTP 端口发现
	keyPidStatus         = "pid_status"           // 进程状态，带有 metric 和 pid 参数
	keyHttpStatus        = "http_status"          // HTTP 状态检查，带有 Port 参数
)

// 定义一个映射，将每个监控项的键与对应的处理函数关联
var metricHandlers = map[string]func(context.Context, map[string]string) (any, error){
	keyTcpPortDiscovery:  handlers.WithJSONResponse(handlers.GetTcpPortInfo),  // TCP 端口发现处理函数
	keyUdpPortDiscovery:  handlers.WithJSONResponse(handlers.GetUdpPortInfo),  // UDP 端口发现处理函数
	keyHttpPortDiscovery: handlers.WithJSONResponse(handlers.GetHttpPortInfo), // HTTP 端口发现处理函数
	keyPidStatus:         handlers.WithJSONResponse(handlers.GetPidStatus),    // 进程状态处理函数
	keyHttpStatus:        handlers.WithJSONResponse(handlers.CheckHttpStatus), // HTTP 状态检查处理函数
}

// 定义每个监控项的指标集合
var metrics = metric.MetricSet{
	keyTcpPortDiscovery:  metric.New("获取 TCP 端口信息", nil, false),  // TCP 端口发现指标
	keyUdpPortDiscovery:  metric.New("获取 UDP 端口信息", nil, false),  // UDP 端口发现指标
	keyHttpPortDiscovery: metric.New("获取 HTTP 端口信息", nil, false), // HTTP 端口发现指标

	keyPidStatus: metric.New(
		"获取进程性能信息",
		[]*metric.Param{
			metric.NewParam("metric", "指标名称，如 VmSize、VmRSS、Threads、voluntary、nonvoluntary、minflt/s、majflt/s、kB_rd/s、kB_wr/s、%CPU、%MEM"),
			metric.NewParam("pid", "进程ID"),
		},
		false,
	),

	keyHttpStatus: metric.New(
		"检查 HTTP 端口状态",
		[]*metric.Param{
			metric.NewParam("Port", "指定要检查的 HTTP 端口"),
		},
		false,
	),
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
