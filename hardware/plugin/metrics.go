// 定义所有硬件监控指标及其参数设置
package plugin

import (
	"context"                                 // 用于管理插件中的上下文传递，便于控制执行流程和取消操作
	"log"                                     // 用于在控制台或日志文件中记录日志信息
	"golang.zabbix.com/plugin/hardware/plugin/handlers" // 引入硬件信息的处理程序库，处理不同的监控项
	"golang.zabbix.com/sdk/metric"            // 引入 SDK 的指标模块，定义和管理监控指标
	"golang.zabbix.com/sdk/plugin"            // 引入插件管理模块，用于插件注册和初始化
)

// 定义硬件和操作系统监控项的键名，用于标识每个监控项
const (
	keySystemManufacturer    = "hardware.system.manufacturer"    // 监控系统制造商信息
	keySystemProductName     = "hardware.system.productName"     // 监控系统产品名称
	keySystemSerialNumber    = "hardware.system.serialNumber"    // 监控系统序列号
	keySystemUUID            = "hardware.system.uuid"            // 监控系统的 UUID
	keyBaseboardManufacturer = "hardware.baseboard.manufacturer" // 监控主板制造商信息
	keyBaseboardProductName  = "hardware.baseboard.productName"  // 监控主板产品名称
	keyBaseboardVersion      = "hardware.baseboard.version"      // 监控主板版本信息
	keyBaseboardSerialNumber = "hardware.baseboard.serialNumber" // 监控主板序列号
	keyCPUModel              = "hardware.cpu.model"              // 监控 CPU 型号
	keyCPUUsage              = "hardware.cpu.usage"              // 监控 CPU 使用率
	keyCPUCores              = "hardware.cpu.cores"              // 监控 CPU 核心数
	keyMemoryUsage           = "hardware.memory.usage"           // 监控内存使用率
	keyMemoryTotal           = "hardware.memory.total"           // 监控内存总大小
	keyDiskCapacity          = "hardware.disk.capacity"          // 监控磁盘总容量
	keyDiskUsage             = "hardware.disk.usage"             // 监控磁盘使用率
	keyOSName                = "hardware.os.name"                // 监控操作系统名称
	keyOSType                = "hardware.os.type"                // 监控操作系统类型
	keyOSVersion             = "hardware.os.version"             // 监控操作系统版本
)

// 定义一个映射，将每个监控项的键与对应的处理函数关联，以便动态调用
var metricHandlers = map[string]func(context.Context, map[string]string) (any, error){
	keySystemManufacturer:    handlers.SystemManufacturerHandler,    // 系统制造商处理函数
	keySystemProductName:     handlers.SystemProductNameHandler,     // 系统产品名称处理函数
	keySystemSerialNumber:    handlers.SystemSerialNumberHandler,    // 系统序列号处理函数
	keySystemUUID:            handlers.SystemUUIDHandler,            // 系统 UUID 处理函数
	keyBaseboardManufacturer: handlers.BaseboardManufacturerHandler, // 主板制造商处理函数
	keyBaseboardProductName:  handlers.BaseboardProductNameHandler,  // 主板产品名称处理函数
	keyBaseboardVersion:      handlers.BaseboardVersionHandler,      // 主板版本处理函数
	keyBaseboardSerialNumber: handlers.BaseboardSerialNumberHandler, // 主板序列号处理函数
	keyCPUModel:              handlers.CPUModelHandler,              // CPU 型号处理函数
	keyCPUUsage:              handlers.CPUUsageHandler,              // CPU 使用率处理函数，支持可选参数
	keyCPUCores:              handlers.CPUCoresHandler,              // CPU 核心数处理函数
	keyMemoryUsage:           handlers.MemoryUsageHandler,           // 内存使用率处理函数
	keyMemoryTotal:           handlers.MemoryTotalHandler,           // 内存总大小处理函数
	keyDiskCapacity:          handlers.DiskCapacityHandler,          // 磁盘总容量处理函数，支持可选参数
	keyDiskUsage:             handlers.DiskUsageHandler,             // 磁盘使用率处理函数，支持可选参数
	keyOSName:                handlers.OSNameHandler,                // 操作系统名称处理函数
	keyOSType:                handlers.OSTypeHandler,                // 操作系统类型处理函数
	keyOSVersion:             handlers.OSVersionHandler,             // 操作系统版本处理函数
}

// 初始化每个监控项的指标集合，定义指标的名称、参数和是否聚合
var metrics = metric.MetricSet{
	keySystemManufacturer: metric.New("系统制造商信息", nil, false),   // 系统制造商指标，不需要参数
	keySystemProductName:  metric.New("系统产品名称信息", nil, false), // 系统产品名称指标，不需要参数
	keySystemSerialNumber: metric.New("系统序列号信息", nil, false),   // 系统序列号指标，不需要参数
	keySystemUUID:         metric.New("系统 UUID 信息", nil, false),   // 系统 UUID 指标，不需要参数
	keyBaseboardManufacturer: metric.New("主板制造商信息", nil, false), // 主板制造商指标，不需要参数
	keyBaseboardProductName:  metric.New("主板产品名称信息", nil, false), // 主板产品名称指标，不需要参数
	keyBaseboardVersion:      metric.New("主板版本信息", nil, false),   // 主板版本指标，不需要参数
	keyBaseboardSerialNumber: metric.New("主板序列号信息", nil, false),   // 主板序列号指标，不需要参数
	keyCPUModel:              metric.New("CPU 型号信息", nil, false),   // CPU 型号指标，不需要参数
	keyCPUUsage: metric.New( // 定义 CPU 使用率指标，带有一个可选参数
		"CPU 使用率信息", 
		[]*metric.Param{
			metric.NewParam("duration", "采样时间，如 '2s'，默认 1s"), // 定义采样时间参数
		}, 
		false,
	),
	keyCPUCores: metric.New("CPU 核心数信息", nil, false),             // CPU 核心数指标，不需要参数
	keyMemoryUsage: metric.New("内存使用率信息", nil, false),           // 内存使用率指标，不需要参数
	keyMemoryTotal: metric.New("内存总大小信息", nil, false),           // 内存总大小指标，不需要参数
	keyDiskCapacity: metric.New( // 定义磁盘总容量指标，带有一个可选参数
		"磁盘总容量信息", 
		[]*metric.Param{
			metric.NewParam("path", "磁盘路径，例如 '/' 或 '/data'"), // 定义磁盘路径参数
		}, 
		false,
	),
	keyDiskUsage: metric.New( // 定义磁盘使用率指标，带有一个可选参数
		"磁盘使用率信息", 
		[]*metric.Param{
			metric.NewParam("path", "磁盘路径，例如 '/' 或 '/data'"), // 定义磁盘路径参数
		}, 
		false,
	),
	keyOSName: metric.New("操作系统名称信息", nil, false),            // 操作系统名称指标，不需要参数
	keyOSType: metric.New("操作系统类型信息", nil, false),            // 操作系统类型指标，不需要参数
	keyOSVersion: metric.New("操作系统版本信息", nil, false),         // 操作系统版本指标，不需要参数
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
