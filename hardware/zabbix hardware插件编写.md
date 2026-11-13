# zabbix Hardware 插件编写

> 适用版本：Zabbix Agent 2 8.0 / golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3
> 代码目录：`hardware/`
> 最后更新：2026-11-13

> **SDK 版本说明**：本文档对应 `golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3`。
> 从该版本开始，`plugin/flag.HandleFlags()` 不再接收参数，返回 `(*Arguments, error)`；
> 入口程序通过 `flag.DecideActionFromFlags(args, &plugin.Impl, pluginInfo, nil)` 处理 `-h/-V/-t/-v`，
> 并使用 `errors.Is(err, errs.ErrExitGracefully)` 判断正常退出。旧版 7 参数 `HandleFlags` 写法已不适用。
>
> **构建要求**：Go 1.25.12 及以上（SDK 的 `go.mod` 要求）。当前插件 `go.mod` 声明 `go 1.27.4`，
> Go 工具链会自动下载并使用对应版本，首次构建需要可访问 `GOPROXY`。

## 1. 插件简介

`Hardware` 插件采集服务器硬件与操作系统基础信息，数据来源：
- `github.com/jaypipes/ghw`：系统/主板/物理磁盘信息；
- `github.com/shirou/gopsutil`：CPU、内存、磁盘使用率、操作系统信息。

适用于资产信息采集、容量趋势和硬件变更发现。

## 2. 代码结构

```text
hardware/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── hardware.conf                        # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 18 个硬件指标注册
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        └── handlers_hardware.go         # 硬件/系统信息采集处理函数
```

## 3. 监控项与返回值

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `hardware.system.manufacturer` | 无 | 字符串 | 系统制造商（ghw.Product） |
| `hardware.system.productName` | 无 | 字符串 | 系统产品名称 |
| `hardware.system.serialNumber` | 无 | 字符串 | 系统序列号 |
| `hardware.system.uuid` | 无 | 字符串 | 系统 UUID |
| `hardware.baseboard.manufacturer` | 无 | 字符串 | 主板制造商 |
| `hardware.baseboard.productName` | 无 | 字符串 | 主板产品名称 |
| `hardware.baseboard.version` | 无 | 字符串 | 主板版本 |
| `hardware.baseboard.serialNumber` | 无 | 字符串 | 主板序列号 |
| `hardware.cpu.model` | 无 | 字符串 | CPU 型号 |
| `hardware.cpu.usage` | `duration`（可选，默认 `1s`） | 字符串，保留 2 位小数 | CPU 总使用率百分比 |
| `hardware.cpu.cores` | 无 | 字符串整数 | CPU 逻辑核心数（`cpu.Info()` 条目数） |
| `hardware.memory.usage` | 无 | 字符串，保留 2 位小数 | 内存使用率百分比 |
| `hardware.memory.total` | 无 | 字符串 | 内存总容量，自动选择 `KB/MB/GB/TB` |
| `hardware.disk.capacity` | `path`（当前未参与计算） | 字符串 | 物理磁盘总容量，自动选择 `KB/MB/GB/TB` |
| `hardware.disk.usage` | `path`（当前未参与计算） | 字符串，保留 2 位小数 | 所有有效分区聚合使用率百分比 |
| `hardware.os.name` | 无 | 字符串 | 操作系统名称（`Platform`） |
| `hardware.os.type` | 无 | 字符串 | 操作系统类型（`PlatformFamily`） |
| `hardware.os.version` | 无 | 字符串 | 操作系统版本（`PlatformVersion`） |

采集失败时多数处理函数返回 `"未知"` 并附带错误信息。

## 4. 配置文件

配置文件默认安装到 Zabbix Agent 2 的插件配置目录，需通过 `Include=` 指令被主配置加载。

### 4.1 配置项说明

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.Hardware.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.Hardware.Timeout` | 否 | 插件调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.Hardware.Sessions.*.Username` | 否 | 命名会话用户名（当前业务代码未使用） |
| `Plugins.Hardware.Sessions.*.Password` | 否 | 命名会话密码（当前业务代码未使用） |
| `Plugins.Hardware.Default.Username` | 否 | 默认会话用户名（当前业务代码未使用） |
| `Plugins.Hardware.Default.Password` | 否 | 默认会话密码（当前业务代码未使用） |

### 4.2 完整配置示例

```ini
### 配置项: Plugins.Hardware.System.Path
#   插件可执行文件的路径，指向外部插件的路径。

# 必选: 是
# 默认:
Plugins.Hardware.System.Path=/usr/local/zabbix/sbin/zabbix-agent2-plugin/zabbix-agent2-plugin-hardware

### 配置项: Plugins.Hardware.Timeout
#   插件调用地址时的默认超时时间设置，如果未指定超时时间，则使用全局超时时间。

# 必选: 否
# 范围: 1-30
# 默认:
Plugins.Hardware.Timeout=30

### 配置项: Plugins.Hardware.Sessions.*.Username
#   会话参数，指定的用户名，这些用户名在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.Hardware.Sessions.*.Username=

### 配置项: Plugins.Hardware.Sessions.*.Password
#   会话参数，指定的密码，这些密码在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.Hardware.Sessions.*.Password=

### 配置项: Plugins.Hardware.Default.Username
#   默认会话参数，指定的用户名，这些用户名在代码中硬编码。
#   如果没有指定其他用户名，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.Hardware.Default.Username=

### 配置项: Plugins.Hardware.Default.Password
#   默认会话参数，指定的密码，这些密码在代码中硬编码。
#   如果没有指定其他密码，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.Hardware.Default.Password=
```

## 5. 代码清单

### 5.1 `main.go`

```go
// main.go 入口程序：负责插件的初始化、启动和管理
package main

import (
	"errors" // 用于错误处理
	"fmt"    // 用于格式化输出
	"os"     // 用于操作系统相关操作

	"golang.zabbix.com/plugin/hardware/plugin" // 导入硬件插件包
	"golang.zabbix.com/sdk/errs"               // 导入 Zabbix SDK 错误包
	sdkplugin "golang.zabbix.com/sdk/plugin"   // 导入 Zabbix SDK 插件包
	"golang.zabbix.com/sdk/plugin/container"   // 导入 Zabbix SDK 容器包
	"golang.zabbix.com/sdk/plugin/flag"        // 导入 Zabbix SDK 标志包
)

// 定义版权信息常量
const COPYRIGHT_MESSAGE = `
Copyright 2001-%d Zabbix SIA
by xiaonuo
`

// 定义全局变量，用于插件版本信息
var (
	PLUGIN_VERSION_MAJOR = 8
	PLUGIN_VERSION_MINOR = 0
	PLUGIN_VERSION_PATCH = 0
	PLUGIN_VERSION_RC    = ""
	PLUGIN_LICENSE_YEAR  = 2026
)

// main 函数是程序的入口
func main() {
	// 解析命令行标志（-h、-V、-t、-v）
	args, err := flag.HandleFlags()
	if err != nil {
		// 如果是正常退出（例如 -h/-V），直接返回
		if errors.Is(err, errs.ErrExitGracefully) {
			return
		}
		panic(fmt.Sprintf("处理命令行参数失败: %s", err.Error()))
	}

	// 插件静态信息，用于 -h/-V 输出
	pluginInfo := &sdkplugin.Info{
		Name:             plugin.Name,
		BinName:          os.Args[0],
		CopyrightMessage: fmt.Sprintf(COPYRIGHT_MESSAGE, PLUGIN_LICENSE_YEAR),
		MajorVersion:     PLUGIN_VERSION_MAJOR,
		MinorVersion:     PLUGIN_VERSION_MINOR,
		PatchVersion:     PLUGIN_VERSION_PATCH,
		Alphatag:         PLUGIN_VERSION_RC,
	}

	// 根据命令行标志执行相应动作（显示版本、帮助或测试监控项）
	err = flag.DecideActionFromFlags(args, &plugin.Impl, pluginInfo, nil)
	if err != nil {
		if errors.Is(err, errs.ErrExitGracefully) {
			return
		}
		panic(fmt.Sprintf("处理插件命令行动作失败: %s", err.Error()))
	}

	// 创建新的插件处理器，用于管理插件的生命周期
	h, err := container.NewHandler(plugin.Name)
	if err != nil {
		// 如果创建处理器失败，打印错误并终止程序
		panic(fmt.Sprintf("创建插件处理器失败: %s", err.Error()))
	}

	// 将插件的日志处理器设置为新创建的处理器
	plugin.Impl.Logger = h

	// 执行插件的处理逻辑
	err = h.Execute()
	if err != nil {
		panic(fmt.Sprintf("执行插件处理器失败: %s", err.Error()))
	}
}
```

### 5.2 `plugin/config.go`

```go
// 定义插件的配置结构和验证方法
package plugin

import (
    "golang.zabbix.com/sdk/conf"   // 导入配置解析模块
    "golang.zabbix.com/sdk/errs"   // 导入错误处理模块
    "golang.zabbix.com/sdk/plugin" // 导入插件模块
)

// 定义 session 结构体，用于会话信息
type session struct {
    Username string `conf:"optional"` // 用户名，可选
    Password string `conf:"optional"` // 密码，可选
}

// 定义 pluginConfig 结构体，用于存储插件配置
type pluginConfig struct {
    Timeout              int               `conf:"optional,range=1:30"` // 超时时间配置
    Sessions             map[string]session `conf:"optional"`           // 会话集合
    Default              session           `conf:"optional"`            // 默认会话
}

// Configure：解析并初始化插件的配置
func (p *Plugin) Configure(global *plugin.GlobalOptions, options any) {
    pConfig := &pluginConfig{} // 初始化配置结构体

    // 使用 SDK 解析传入的配置选项
    err := conf.Unmarshal(options, pConfig)
    if err != nil {
        p.Errf("配置解析失败: %s", err.Error()) // 输出错误日志
        return
    }
    
    // 如果超时时间未设置，则使用全局超时时间
    if pConfig.Timeout == 0 {
        pConfig.Timeout = global.Timeout
    }

    // 保存解析后的配置
    p.config = pConfig

}

// Validate：验证配置是否正确
func (*Plugin) Validate(options any) error {
    var opts pluginConfig // 初始化临时配置结构体

    // 使用 SDK 解析配置选项
    err := conf.Unmarshal(options, &opts)
    if err != nil {
        return errs.Wrap(err, "配置解析失败") // 返回解析错误
    }

    return nil // 配置验证通过
}

// contains 函数检查字符串是否存在于给定的数组中
func contains(s []string, e string) bool {
    for _, v := range s {
        if v == e {
            return true
        }
    }
    return false
}
```

### 5.3 `plugin/metrics.go`

```go
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
```

### 5.4 `plugin/plugin.go`

```go
// plugin.go：实现插件的主要逻辑，包括启动、停止和导出监控数据
package plugin

import (
    "context"                       // 用于上下文管理
    "log"                           // 用于日志记录
    "time"                          // 用于时间管理和超时控制
    "golang.zabbix.com/sdk/errs"    // 错误处理模块
    "golang.zabbix.com/sdk/metric"  // 导入 Zabbix SDK 的指标模块
    "golang.zabbix.com/sdk/plugin"  // 导入 Zabbix SDK 的插件模块
    "golang.zabbix.com/sdk/zbxerr"  // 导入 Zabbix SDK 的错误处理模块
)

// 定义插件名称常量
const Name = "Hardware"

// 定义全局插件实例
var Impl Plugin

// Plugin 结构体：存储插件的配置和指标
type Plugin struct {
    plugin.Base                      // 嵌入基础插件结构体，提供通用插件方法
    config  *pluginConfig             // 插件的配置项
    metrics map[string]*metric.Metric // 指标集合，用于管理不同的监控项
}

// init 函数在程序启动时运行，设置日志输出格式
func init() {
    log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// Export：根据传入的监控键和参数导出监控数据
func (p *Plugin) Export(key string, rawParams []string, pluginCtx plugin.ContextProvider) (any, error) {
    // 获取指定的指标配置项
    metricSet, exists := p.metrics[key]
    if !exists {
        log.Printf("不支持的监控项: %s", key)
        return nil, zbxerr.ErrorUnsupportedMetric // 返回不支持的监控项错误
    }

    // 解析监控项参数
    params, _, hc, err := metricSet.EvalParams(rawParams, p.config.Sessions)
    if err != nil {
        log.Printf("解析参数错误: %v", err)
        return nil, err
    }

    // 使用插件配置中的默认设置来初始化参数
    err = metric.SetDefaults(params, hc, p.config.Default)
    if err != nil {
        log.Printf("设置密钥默认值时出错 %s: %v", key, err)
        return nil, err
    }

    // 获取监控项对应的处理函数
    handleMetric := getHandlerFunc(key)
    if handleMetric == nil {
        log.Printf("未找到处理函数: %s", key)
        return nil, zbxerr.ErrorUnsupportedMetric
    }

    // 设置上下文超时，确保查询不会无限制等待
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(pluginCtx.Timeout()))
    defer cancel() // 确保上下文在函数结束时取消

    // 执行处理函数并返回结果
    result, err := handleMetric(ctx, params)
    if err != nil {
        log.Printf("处理监控项 %s 时出错: %v", key, err)
        return nil, err
    }

    log.Printf("导出结果: %s = %v", key, result) // 记录导出结果
    return result, nil // 返回结果和 nil 表示成功
}

// Start：启动插件，初始化配置和指标
func (p *Plugin) Start() {
    p.config = &pluginConfig{} // 初始化配置结构体
    p.metrics = metrics        // 初始化指标集合
    if p.metrics == nil {
        wrappedErr := errs.Wrap(nil, "插件启动时无法初始化指标")
        log.Println(wrappedErr) // 如果指标为空，记录致命错误并退出
    }
    log.Println("插件启动成功") // 输出启动成功日志
}

// Stop：停止插件，释放资源
func (p *Plugin) Stop() {
    p.metrics = nil // 清空指标集合，释放内存
    p.config = nil  // 清空配置项，释放内存
    log.Println("插件已停止") // 输出停止日志
}
```

### 5.5 `plugin/handlers/handlers_hardware.go`

```go
// 定义硬件信息查询的处理函数
package handlers

import (
	"context" // 提供上下文管理功能，便于控制操作的生命周期
	"fmt"     // 用于格式化字符串输出
	"math"    // 引入 math 包以便进行四舍五入
	"os"      // 提供操作系统功能，如获取命令行参数和设置环境变量
	"os/exec" // 提供执行系统命令的功能
	"strconv" // 提供字符串转换功能
	"strings" // 提供字符串操作功能
	"time"    // 提供时间操作，如设置采样时间间隔

	"github.com/jaypipes/ghw"         // 硬件信息库，用于获取系统硬件信息
	"github.com/shirou/gopsutil/cpu"  // 系统信息库，用于获取 CPU 相关信息
	"github.com/shirou/gopsutil/disk" // 系统信息库，用于获取磁盘相关信息
	"github.com/shirou/gopsutil/host" // 系统信息库，用于获取操作系统相关信息
	"github.com/shirou/gopsutil/mem"  // 系统信息库，用于获取内存相关信息
)

// init 函数在程序加载时执行，设置环境变量以禁用 ghw 库的警告信息
func init() {
	os.Setenv("GHW_DISABLE_WARNINGS", "1") // 禁用 ghw 库的警告信息输出
}

// convertSize 自动转换单位的函数
// 输入参数：size（以字节为单位的容量值）
// 返回值：转换后的容量值（float64）和单位（string）
func convertSize(size float64) (float64, string) {
	var unit string           // 用于存储单位（如 TB、GB、MB、KB）
	var convertedSize float64 // 用于存储转换后的容量值

	// 根据容量大小选择合适的单位
	switch {
	case size >= 1024*1024*1024*1024: // 如果容量大于等于 1 TB
		convertedSize = size / (1024 * 1024 * 1024 * 1024) // 转换为 TB
		unit = "TB"
	case size >= 1024*1024*1024: // 如果容量大于等于 1 GB
		convertedSize = size / (1024 * 1024 * 1024) // 转换为 GB
		unit = "GB"
	case size >= 1024*1024: // 如果容量大于等于 1 MB
		convertedSize = size / (1024 * 1024) // 转换为 MB
		unit = "MB"
	default: // 如果容量小于 1 MB，使用 KB
		convertedSize = size / 1024 // 转换为 KB
		unit = "KB"
	}

	return convertedSize, unit // 返回转换后的容量值和单位
}

// 系统制造商处理器，返回系统制造商信息
func SystemManufacturerHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Product() // 使用 ghw 库获取系统产品信息
	if err != nil {
		return "未知", err // 若发生错误，返回“未知”和错误信息
	}
	return info.Vendor, nil // 返回系统制造商信息
}

// 系统产品名称处理器，返回系统的产品名称
func SystemProductNameHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Product() // 获取系统产品名称信息
	if err != nil {
		return "未知", err // 返回错误时显示“未知”并附带错误信息
	}
	return info.Name, nil // 返回系统产品名称
}

// 系统序列号处理器，返回系统序列号
func SystemSerialNumberHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Product() // 获取系统序列号信息
	if err != nil {
		return "未知", err // 发生错误则返回“未知”和错误信息
	}
	return info.SerialNumber, nil // 返回系统序列号
}

// 系统 UUID 处理器，返回系统的 UUID
func SystemUUIDHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Product() // 获取系统 UUID 信息
	if err != nil {
		return "未知", err // 返回“未知”错误信息
	}
	return info.UUID, nil // 返回系统 UUID
}

// 主板制造商处理器，返回主板制造商信息
func BaseboardManufacturerHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Baseboard() // 获取主板制造商信息
	if err != nil {
		return "未知", err // 错误处理
	}
	return info.Vendor, nil // 返回主板制造商
}

// 主板产品名称处理器，返回主板产品名称
func BaseboardProductNameHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Baseboard() // 获取主板产品名称信息
	if err != nil {
		return "未知", err // 返回错误信息
	}
	return info.Product, nil // 返回主板产品名称
}

// 主板版本处理器，返回主板版本
func BaseboardVersionHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Baseboard() // 获取主板版本信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.Version, nil // 返回主板版本
}

// 主板序列号处理器，返回主板序列号
func BaseboardSerialNumberHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Baseboard() // 获取主板序列号信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.SerialNumber, nil // 返回主板序列号
}

// CPU 型号处理器，返回 CPU 型号信息
func CPUModelHandler(ctx context.Context, params map[string]string) (any, error) {
	cpuInfo, err := cpu.Info() // 获取 CPU 信息
	if err != nil || len(cpuInfo) == 0 {
		return "未知", err // 错误时返回“未知”
	}
	return cpuInfo[0].ModelName, nil // 返回第一个 CPU 的型号
}

// CPU 使用率处理器，返回 CPU 使用率
func CPUUsageHandler(ctx context.Context, params map[string]string) (any, error) {
	sampleDuration := 1 * time.Second           // 设置默认采样时间为 1 秒
	if duration, ok := params["duration"]; ok { // 检查是否有自定义采样时间
		if d, err := time.ParseDuration(duration); err == nil {
			sampleDuration = d // 使用用户指定的采样时间
		}
	}
	usage, err := cpu.Percent(sampleDuration, false) // 获取 CPU 使用率
	if err != nil || len(usage) == 0 {
		return "未知", err // 错误时返回“未知”
	}
	return fmt.Sprintf("%.2f", usage[0]), nil // 格式化使用率并返回
}

// CPU 核心数处理器，返回 CPU 核心数量
func CPUCoresHandler(ctx context.Context, params map[string]string) (any, error) {
	cpuInfo, err := cpu.Info() // 获取 CPU 信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return fmt.Sprintf("%d", len(cpuInfo)), nil // 返回 CPU 核心数
}

// 内存使用率处理器，返回内存使用率
func MemoryUsageHandler(ctx context.Context, params map[string]string) (any, error) {
	memInfo, err := mem.VirtualMemory() // 获取内存使用率信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return fmt.Sprintf("%.2f", memInfo.UsedPercent), nil // 格式化内存使用率并返回
}

// MemoryTotalHandler 返回内存总大小并自动转换单位
// 参数：
// - ctx：上下文，用于控制命令执行的超时或取消
// - params：包含路径等信息的参数（此处未用到路径）
// 返回值：格式化后的内存大小信息（如 "16 GB"）和错误信息（如果有）
func MemoryTotalHandler(ctx context.Context, params map[string]string) (any, error) {
	// 获取系统内存信息
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return "未知", err // 如果获取失败，返回“未知”及错误信息
	}

	// 获取内存总大小（以字节为单位）
	totalMemory := float64(memInfo.Total)

	// 使用 convertSize 函数转换内存大小
	convertedSize, unit := convertSize(totalMemory)

	// 四舍五入并转为整数
	roundedSize := int(math.Round(convertedSize))

	// 格式化结果为整数并附加单位
	return fmt.Sprintf("%d %s", roundedSize, unit), nil
}

// DiskCapacityHandler 返回磁盘总容量并自动转换单位
// 参数：
// - ctx：上下文，用于控制命令执行
// - params：包含路径等信息的参数（此处未用到路径）
// 返回值：格式化后的容量信息（如 "500 GB"）和错误信息（如果有）
func DiskCapacityHandler(ctx context.Context, params map[string]string) (any, error) {
	// 首先尝试使用 ghw 库获取物理磁盘信息
	block, err := ghw.Block()
	if err != nil {
		return "未知", err
	}

	var total uint64 = 0
	var diskCount int = 0
	processedDevices := make(map[string]bool) // 用于跟踪已处理的设备

	// 只计算物理磁盘的容量，不包括分区
	for _, disk := range block.Disks {
		// 跳过 CD-ROM 等只读设备
		if disk.DriveType.String() == "cdrom" {
			continue
		}
		// 跳过虚拟设备，但保留 RBD 设备
		if disk.DriveType.String() == "loop" && !strings.Contains(disk.Name, "rbd") {
			continue
		}
		// 跳过 LVM 设备（dm-* 设备）
		if strings.HasPrefix(disk.Name, "dm-") {
			continue
		}
		// 跳过 CD-ROM 设备（sr* 设备）
		if strings.HasPrefix(disk.Name, "sr") {
			continue
		}
		// 跳过已处理的设备
		if processedDevices[disk.Name] {
			continue
		}

		total += disk.SizeBytes
		diskCount++
		processedDevices[disk.Name] = true
	}

	// 如果检测到的磁盘数量太少（少于1个），可能 ghw 库没有正确识别所有磁盘
	// 在这种情况下，我们使用备选方案（仅限 Linux）
	if diskCount < 1 {
		// 检查操作系统类型
		hostInfo, err := host.Info()
		if err == nil {
			switch hostInfo.PlatformFamily {
			case "linux":
				// 在 Linux 上使用 lsblk 命令作为备选方案，但过滤掉 LVM 设备和 CD-ROM
				cmd := exec.Command("lsblk", "-b", "-d", "-o", "SIZE,NAME", "--noheadings")
				output, err := cmd.Output()
				if err == nil {
					lines := strings.Split(strings.TrimSpace(string(output)), "\n")
					var lsblkTotal uint64 = 0

					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line != "" {
							parts := strings.Fields(line)
							if len(parts) >= 2 {
								deviceName := parts[1]
								// 跳过 LVM 设备
								if strings.HasPrefix(deviceName, "dm-") {
									continue
								}
								// 跳过 CD-ROM 设备
								if strings.HasPrefix(deviceName, "sr") {
									continue
								}
								if size, err := strconv.ParseUint(parts[0], 10, 64); err == nil {
									lsblkTotal += size
								}
							}
						}
					}
					// 如果 lsblk 的结果更大，使用 lsblk 的结果
					if lsblkTotal > total {
						total = lsblkTotal
					}
				}
			case "windows":
				// 在 Windows 上使用 gopsutil 的磁盘信息作为备选方案
				partitions, err := disk.Partitions(false)
				if err == nil {
					var winTotal uint64 = 0
					processedDevices := make(map[string]bool)
					for _, p := range partitions {
						if processedDevices[p.Device] {
							continue
						}
						usage, err := disk.Usage(p.Mountpoint)
						if err == nil {
							winTotal += usage.Total
							processedDevices[p.Device] = true
						}
					}
					// 如果 Windows 方法的结果更大，使用 Windows 的结果
					if winTotal > total {
						total = winTotal
					}
				}
			}
		}
	}

	convertedSize, unit := convertSize(float64(total))
	roundedSize := int(math.Round(convertedSize))
	return fmt.Sprintf("%d %s", roundedSize, unit), nil
}

// 磁盘使用率处理器，返回指定路径的磁盘使用率
func DiskUsageHandler(ctx context.Context, params map[string]string) (any, error) {
	// 获取所有挂载点
	partitions, err := disk.Partitions(false)
	if err != nil {
		return "未知", err
	}

	var total uint64 = 0
	var used uint64 = 0
	processedDevices := make(map[string]bool) // 用于跟踪已处理的设备

	// 遍历所有分区，但只计算每个物理设备一次
	for _, p := range partitions {
		// 跳过已处理的设备
		if processedDevices[p.Device] {
			continue
		}

		usage, err := disk.Usage(p.Mountpoint)
		if err == nil {
			total += usage.Total
			used += usage.Used
			processedDevices[p.Device] = true
		}
	}

	if total == 0 {
		return "未知", fmt.Errorf("无有效磁盘分区")
	}
	percent := (float64(used) / float64(total)) * 100
	return fmt.Sprintf("%.2f", percent), nil
}

// 操作系统名称处理器，返回操作系统的名称
func OSNameHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := host.Info() // 获取操作系统信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.Platform, nil // 返回操作系统名称
}

// 操作系统类型处理器，返回操作系统类型
func OSTypeHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := host.Info() // 获取操作系统信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.PlatformFamily, nil // 返回操作系统类型
}

// 操作系统版本处理器，返回操作系统版本
func OSVersionHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := host.Info() // 获取操作系统信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.PlatformVersion, nil // 返回操作系统版本
}
```

## 6. 编译与构建

```bash
cd hardware

# 如需从零初始化模块（会删除已有依赖锁定）
# rm -rf go.mod go.sum zabbix-agent2-*

go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/hardware
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))

# Windows 交叉编译
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o zabbix-agent2-plugin-hardware.exe .
```

构建产物：`hardware/zabbix-agent2-plugin-hardware`（Windows 为 `.exe`）。

## 7. 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-hardware /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp hardware.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-hardware -V
./zabbix-agent2-plugin-hardware -t hardware.system.manufacturer
./zabbix-agent2-plugin-hardware -t 'hardware.cpu.usage[2s]'
./zabbix-agent2-plugin-hardware -t hardware.memory.total
./zabbix-agent2-plugin-hardware -t hardware.disk.capacity
```

## 8. 注意事项

- `hardware.cpu.usage` 默认会阻塞采样 1 秒，可通过参数调整，例如 `hardware.cpu.usage[5s]`；采样时间过长时需同步调大插件 Timeout。
- `hardware.disk.capacity` 与 `hardware.disk.usage` 虽然声明了 `path` 参数，但当前实现未使用该参数：容量统计所有物理磁盘，使用率聚合所有有效分区。
- 容量类指标会自动换算单位并四舍五入为整数，例如 `19 GB`、`2 TB`。
- 部分硬件信息在虚拟机/容器中可能返回空值或 `"未知"`；Linux 下读取 DMI/SMBIOS 信息通常需要 root。
- Windows 构建时建议同时生成 `windres` 资源，以便 `-V` 输出文件版本信息（见 Makefile）。
