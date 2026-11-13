# zabbix Ports 插件编写

> 适用版本：Zabbix Agent 2 8.0 / golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3
> 代码目录：`ports/`
> 最后更新：2026-11-13

> **SDK 版本说明**：本文档对应 `golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3`。
> 从该版本开始，`plugin/flag.HandleFlags()` 不再接收参数，返回 `(*Arguments, error)`；
> 入口程序通过 `flag.DecideActionFromFlags(args, &plugin.Impl, pluginInfo, nil)` 处理 `-h/-V/-t/-v`，
> 并使用 `errors.Is(err, errs.ErrExitGracefully)` 判断正常退出。旧版 7 参数 `HandleFlags` 写法已不适用。
>
> **构建要求**：Go 1.25.12 及以上（SDK 的 `go.mod` 要求）。当前插件 `go.mod` 声明 `go 1.27.4`，
> Go 工具链会自动下载并使用对应版本，首次构建需要可访问 `GOPROXY`。

## 1. 插件简介

`Ports` 插件基于 Linux `/proc` 实现端口与进程监控，提供：
- TCP/UDP/HTTP 监听端口自动发现（LLD）；
- 指定进程 PID 的 CPU、内存、I/O、缺页、上下文切换等性能指标；
- 指定本地端口的 HTTP 可访问性检查。

端口发现采用“先解析 `/proc/net/tcp*` 的 inode→端口映射，再反查 `/proc/<pid>/fd`”的方式，并支持并发扫描与端口去重，避免遍历全部 socket。

## 2. 代码结构

```text
ports/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── ports.conf                           # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
└── plugin/
    ├── config.go                        # 插件配置结构、排除进程配置
    ├── metrics.go                       # 5 个指标注册
    ├── plugin.go                        # 插件生命周期与 Export 分发
    ├── params/
    │   └── params.go                    # 会话参数与排除进程列表
    └── handlers/
        ├── handlers_base.go             # HandlerFunc 与 WithJSONResponse
        ├── handlers_middlewares.go      # 凭据校验中间件
        ├── handlers_ports_discovery.go  # TCP/UDP/HTTP 端口发现
        ├── handlers_pid_status.go       # 进程性能指标采集
        └── handlers_http_status.go      # HTTP 状态检查
```

## 3. 监控项与返回值

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `ports.tcp.discovery` | 无 | JSON 数组字符串 | 发现处于 LISTEN 的 TCP 端口及所属进程 |
| `ports.udp.discovery` | 无 | JSON 数组字符串 | 发现 UDP 端口及所属进程 |
| `ports.http.discovery` | 无 | JSON 数组字符串 | 在 TCP 监听端口中筛选能返回 HTTP 响应头的端口 |
| `pid_status` | `metric`、`pid` | 数值字符串 | 指定进程的性能指标 |
| `http_status` | `Port` | `1` / `0` | 本地端口 HTTP 检查，响应头非空返回 `1` |

端口发现输出字段：

| 字段 | 说明 |
|---|---|
| `{#PROCESS_NAME}` | 进程可执行文件名 |
| `{#PORT}` | 监听端口 |
| `{#PID}` | 进程 ID |
| `{#WORK_DIR}` | 进程工作目录 |
| `{#EXE_PATH}` | 可执行文件完整路径 |
| `{#USERNAME}` | 运行用户 |

`pid_status` 支持的 `metric` 值：

| metric | 说明 |
|---|---|
| `%CPU` | 进程 CPU 使用率（基于两次采样增量） |
| `%MEM` | 进程物理内存占系统内存百分比 |
| `VmSize` / `vmsize` | 虚拟内存大小 |
| `VmRSS` / `vmrss` / `rss` | 物理内存大小 |
| `Threads` / `threads` / `线程数` | 线程数 |
| `minflt/s` | 每秒次要缺页次数 |
| `majflt/s` | 每秒主要缺页次数 |
| `kB_rd/s` | 每秒磁盘读取量（KB） |
| `kB_wr/s` | 每秒磁盘写入量（KB） |
| `voluntary` | 每秒主动上下文切换次数 |
| `nonvoluntary` | 每秒被动上下文切换次数 |

同时支持直接传入标准中文描述（见 `handlers_pid_status.go` 的 `pidstatDict`）。

## 4. 配置文件

配置文件默认安装到 Zabbix Agent 2 的插件配置目录，需通过 `Include=` 指令被主配置加载。

### 4.1 配置项说明

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.Ports.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.Ports.Timeout` | 否 | 插件调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.Ports.Sessions.*.Username` | 否 | 命名会话用户名（当前业务代码未使用） |
| `Plugins.Ports.Sessions.*.Password` | 否 | 命名会话密码（当前业务代码未使用） |
| `Plugins.Ports.Default.Username` | 否 | 默认会话用户名（当前业务代码未使用） |
| `Plugins.Ports.Default.Password` | 否 | 默认会话密码（当前业务代码未使用） |

`Plugins.Ports.ExcludedProcesses` 用于追加排除进程，多个名称用英文逗号分隔。除该配置外，代码还内置了系统进程和常见监控/网络进程排除列表（`systemd`、`zabbix`、`prometheus`、`node_exporter`、`smartdns`、`rpcbind` 等）。

### 4.2 完整配置示例

```ini
### 配置项: Plugins.Ports.System.Path
#   插件可执行文件的路径，指向外部插件的路径。

# 必选: 是
# 默认:
Plugins.Ports.System.Path=/usr/local/zabbix/sbin/zabbix-agent2-plugin/zabbix-agent2-plugin-ports

### 配置项: Plugins.Ports.Timeout
#   插件调用地址时的默认超时时间设置，如果未指定超时时间，则使用全局超时时间。

# 必选: 否
# 范围: 1-30
# 默认:
# Plugins.Ports.Timeout=<全局超时时间>

### 配置项: Plugins.Ports.Sessions.*.Username
#   会话参数，指定的用户名，这些用户名在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.Ports.Sessions.*.Username=

### 配置项: Plugins.Ports.Sessions.*.Password
#   会话参数，指定的密码，这些密码在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.Ports.Sessions.*.Password=

### 配置项: Plugins.Ports.Default.Username
#   默认会话参数，指定的用户名，这些用户名在代码中硬编码。
#   如果没有指定其他用户名，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.Ports.Default.Username=

### 配置项: Plugins.Ports.Default.Password
#   默认会话参数，指定的密码，这些密码在代码中硬编码。
#   如果没有指定其他密码，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.Ports.Default.Password=
#
# 排除特定进程名
Plugins.Ports.ExcludedProcesses=rpcbind
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

	"golang.zabbix.com/plugin/ports/plugin"  // 导入端口插件包
	"golang.zabbix.com/sdk/errs"             // 导入 Zabbix SDK 错误包
	sdkplugin "golang.zabbix.com/sdk/plugin" // 导入 Zabbix SDK 插件包
	"golang.zabbix.com/sdk/plugin/container" // 导入 Zabbix SDK 容器包
	"golang.zabbix.com/sdk/plugin/flag"      // 导入 Zabbix SDK 标志包
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
    "golang.zabbix.com/plugin/ports/plugin/params" // 导入 params 包
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
    ExcludedProcesses    string            `conf:"optional"`            // 排除进程配置项
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
    
    // 解析并设置排除进程的列表
    params.SetExcludedProcesses(pConfig.ExcludedProcesses) // 将排除进程的列表传入参数模块

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
const Name = "Ports"

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
    log.SetFlags(log.LstdFlags | log.Lshortfile) // 日志格式包含日期、时间及文件行号
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

### 5.5 `plugin/params/params.go`

```go
// params.go：定义插件参数和排除进程的设置
package params

import (
    "strings"                      // 字符串处理包
    "golang.zabbix.com/sdk/metric" // 引入 Zabbix SDK 的 metric 包
)

// 定义参数名称的常量
const (
    UsernameParameterName = "Username" // 用户名参数
    PasswordParameterName = "Password" // 密码参数
)

// 定义连接参数的列表，包括用户名和密码
var (
    Params = []*metric.Param{Username, Password} // 基本连接参数列表

    // 用户名参数，默认值为 "Zabbix"
    Username = metric.NewConnParam(
        UsernameParameterName, "模拟用户名参数。",
    ).WithSession().WithDefault("Zabbix")

    // 密码参数
    Password = metric.NewConnParam(PasswordParameterName, "模拟密码参数。")

    // ExcludedProcesses 用于存储配置的排除进程列表
    ExcludedProcesses = []string{"rpcbind"} // 默认排除 "rpcbind" 进程
)

// SetExcludedProcesses 从配置文件设置排除进程列表
func SetExcludedProcesses(excludedProcesses string) {
    if excludedProcesses == "" {
        ExcludedProcesses = []string{"rpcbind"} // 如果未设置则使用默认值
    } else {
        ExcludedProcesses = strings.Split(excludedProcesses, ",") // 用逗号分割
    }
}

// GetExcludedProcesses 返回排除进程列表
func GetExcludedProcesses() []string {
    return ExcludedProcesses
}
```

### 5.6 `plugin/handlers/handlers_base.go`

```go
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
```

### 5.7 `plugin/handlers/handlers_middlewares.go`

```go
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
```

### 5.8 `plugin/handlers/handlers_ports_discovery.go`

```go
// handlers_ports_discovery.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/user"       // 用户信息获取
	"path/filepath" // 用于处理文件路径
	"runtime"       // 获取CPU核心数
	"strconv"       // 字符串转换
	"strings"       // 字符串处理
	"sync"          // 并发相关
	"time"          // 时间相关操作

	"golang.zabbix.com/plugin/ports/plugin/params" // 自定义参数包
)

// JSONOutput 定义Zabbix自动发现所需的JSON数据结构
type JSONOutput struct {
	ProcessName string `json:"{#PROCESS_NAME}"` // 进程名称
	Port        int    `json:"{#PORT}"`         // 监听端口号
	Pid         int    `json:"{#PID}"`          // 进程ID
	WorkDir     string `json:"{#WORK_DIR}"`     // 进程工作目录
	ExePath     string `json:"{#EXE_PATH}"`     // 可执行文件路径
	Username    string `json:"{#USERNAME}"`     // 运行用户
}

// GetTcpPortInfo 处理TCP端口发现请求
func GetTcpPortInfo(ctx context.Context, params map[string]string) (any, error) {
	return getPortInfoJSON("tcp")
}

// GetUdpPortInfo 处理UDP端口发现请求
func GetUdpPortInfo(ctx context.Context, params map[string]string) (any, error) {
	return getPortInfoJSON("udp")
}

// GetHttpPortInfo 处理HTTP端口发现请求（基于TCP端口检查）
func GetHttpPortInfo(ctx context.Context, params map[string]string) (any, error) {
	return getPortInfoJSON("http")
}

// getPortInfoJSON 通用端口信息获取方法
func getPortInfoJSON(protocol string) (any, error) {
	var ports []JSONOutput

	// 根据协议类型选择获取方式
	switch protocol {
	case "tcp":
		ports = getListeningPorts("tcp")
	case "udp":
		ports = getListeningPorts("udp")
	case "http":
		ports = getListeningHttpPorts() // HTTP需要额外检查
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	// 将结果转换为JSON字符串
	jsonData, err := json.Marshal(ports)
	if err != nil {
		return nil, fmt.Errorf("JSON marshal failed: %v", err)
	}
	return string(jsonData), nil
}

// getListeningHttpPorts 获取所有HTTP端口（从TCP端口筛选）
func getListeningHttpPorts() []JSONOutput {
	tcpPorts := getListeningPorts("tcp")
	var httpPorts []JSONOutput

	// 遍历TCP端口检查HTTP响应
	for _, port := range tcpPorts {
		if checkHTTPResponse(port.Port) {
			httpPorts = append(httpPorts, port)
		}
	}
	return httpPorts
}

// getListeningPorts 极致优化：只遍历有监听socket的inode，反查pid，极大减少遍历量
// 实现思路：
// 1. 先解析 /proc/net/tcp*，建立所有监听socket的 inode->port 映射表。
// 2. 统计所有监听socket的inode集合，便于快速判断。
// 3. 并发遍历所有 /proc/[pid]/fd 目录，只查找 fd 指向的 inode 是否在监听集合里。
// 4. 命中后反查出 pid、端口、进程名、用户等信息，端口去重，极大减少无效遍历和I/O。
func getListeningPorts(protocol string) []JSONOutput {
	var results []JSONOutput
	var seen sync.Map // 用于端口去重，确保每个端口只记录一次

	// 1. 解析 /proc/net/tcp*，建立 inode->port 映射
	inodePortMap := buildInodePortMap(protocol)
	if len(inodePortMap) == 0 {
		return results // 没有监听socket，直接返回空
	}

	// 2. 反查所有监听socket的inode属于哪个pid
	// 建立 port->[]inode 映射，便于后续端口去重（可选，当前主要用 inodePortMap）
	portInodes := make(map[int][]string)
	for inode, port := range inodePortMap {
		portInodes[port] = append(portInodes[port], inode)
	}

	// 3. 统计所有监听socket的inode集合，便于快速判断
	inodeSet := make(map[string]struct{})
	for inode := range inodePortMap {
		inodeSet[inode] = struct{}{}
	}

	// 4. 自动调整并发数：默认为CPU核心数一半，向上取整，最小1，最大128。可通过环境变量PORT_DISCOVERY_MAX_CONCURRENCY覆盖。
	maxConcurrency := (runtime.NumCPU() + 1) / 2 // 向上取整
	if maxConcurrency < 1 {
		maxConcurrency = 1 // 保底
	}
	if maxConcurrency > 128 {
		maxConcurrency = 128 // 封顶，防止极端大主机
	}
	// 支持通过环境变量PORT_DISCOVERY_MAX_CONCURRENCY覆盖
	if v := os.Getenv("PORT_DISCOVERY_MAX_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConcurrency = n
		}
	}
	// maxConcurrency为本次端口发现的最大并发数，默认为CPU核心数一半，既能利用多核，又能避免端口发现任务占满所有CPU，影响业务进程。可通过环境变量PORT_DISCOVERY_MAX_CONCURRENCY覆盖。

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency) // 动态并发控制
	resultCh := make(chan JSONOutput, 1024)    // 用于收集结果

	entries, err := os.ReadDir("/proc")
	if err != nil {
		fmt.Printf("读取/proc失败: %v\n", err)
		return results
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // 跳过非目录
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // 跳过非数字目录
		}
		sem <- struct{}{} // 控制并发数
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			defer func() { <-sem }()
			fdPath := fmt.Sprintf("/proc/%d/fd", pid)
			fds, err := os.ReadDir(fdPath)
			if err != nil {
				return // 无权限或目录不存在则跳过
			}
			for _, fdEntry := range fds {
				fdLink := fmt.Sprintf("%s/%s", fdPath, fdEntry.Name())
				link, err := os.Readlink(fdLink)
				if err != nil || !strings.HasPrefix(link, "socket:[") {
					continue // 跳过非socket链接
				}
				inode := link[8 : len(link)-1] // 提取socket inode号
				if _, ok := inodeSet[inode]; !ok {
					continue // 只处理监听socket
				}
				port, ok := inodePortMap[inode]
				if !ok || port == 0 {
					continue // 未找到端口则跳过
				}
				key := strconv.Itoa(port)
				if _, exists := seen.LoadOrStore(key, true); exists {
					continue // 端口去重，确保每个端口只记录一次
				}
				exePath := getProcessExe(pid)
				if isExcludedProcess(exePath) {
					continue // 排除指定进程
				}
				// 命中后收集完整信息
				resultCh <- JSONOutput{
					ProcessName: filepath.Base(exePath),
					Port:        port,
					Pid:         pid,
					WorkDir:     getProcessCwd(pid),
					ExePath:     exePath,
					Username:    getProcessUser(pid),
				}
			}
		}(pid)
	}

	// 等待所有并发任务完成，关闭结果通道
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 主线程收集所有结果
	for r := range resultCh {
		results = append(results, r)
	}
	return results
}

// buildInodePortMap 一次性解析协议文件，返回 inode->port 映射表
func buildInodePortMap(protocol string) map[string]int {
	var protoFiles []string
	switch protocol {
	case "tcp":
		protoFiles = []string{"/proc/net/tcp", "/proc/net/tcp6"}
	case "udp":
		protoFiles = []string{"/proc/net/udp", "/proc/net/udp6"}
	default:
		return map[string]int{}
	}
	m := make(map[string]int)
	for _, file := range protoFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue // 文件读取失败则跳过
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines[1:] { // 跳过标题行
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue // 无效数据行
			}
			// 对于TCP协议检查LISTEN状态（0A为十六进制表示）
			if protocol == "tcp" && len(fields) >= 4 && fields[3] != "0A" {
				continue // 跳过非监听状态连接
			}
			inode := fields[9]
			addrParts := strings.Split(fields[1], ":")
			if len(addrParts) < 2 {
				continue // 无效地址格式
			}
			portHex := addrParts[len(addrParts)-1]
			port, err := strconv.ParseInt(portHex, 16, 32)
			if err != nil {
				continue // 转换失败则跳过
			}
			m[inode] = int(port)
		}
	}
	return m
}

// findPortByInode 通过inode查找端口号（已废弃，仅保留兼容性，实际不再调用）
func findPortByInode(inode string, protocol string) int {
	return 0 // 已不再使用
}

// checkHTTPResponse 验证端口是否响应HTTP请求
func checkHTTPResponse(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second} // 2秒超时
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		return false // 请求失败则非HTTP
	}
	defer resp.Body.Close()
	return len(resp.Header) > 0 // 有响应头则视为HTTP服务
}

// getProcessExe 获取进程的可执行文件路径
func getProcessExe(pid int) string {
	path, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)) // 读取符号链接
	return path
}

// getProcessCwd 获取进程的工作目录
func getProcessCwd(pid int) string {
	path, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return "/" // 默认返回根目录
	}
	return path
}

// getProcessUser 获取进程的运行用户
func getProcessUser(pid int) string {
	content, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}

	// 在status文件中查找Uid行
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				// 通过用户ID获取用户名
				u, err := user.LookupId(fields[1])
				if err == nil {
					return u.Username
				}
			}
		}
	}
	return ""
}

// isExcludedProcess 判断是否是需要排除的进程
func isExcludedProcess(exePath string) bool {
	// 合并预设和自定义排除列表
	excluded := append(params.GetExcludedProcesses(),
		"systemd", "init", "kthreadd", "ksoftirqd", "rcu_sched", "rcu_bh", "bo-agent",
		"migration", "watchdog", "cpuhp", "netns", "khungtaskd", "oom_reaper",
		"kswapd", "fsnotify_mark", "ecryptfs-kthrea", "kauditd", "khubd", "ssh",
		"zabbix", "prometheus", "rpcbind", "smartdns", "cupsd", "dhclient", "master",
		"rpc.statd", "titanagent", "node_exporter", "monitor_manage", "dnsmasq",
	)

	// 提取可执行文件名并检查排除列表
	exeName := filepath.Base(exePath)
	for _, proc := range excluded {
		if strings.Contains(exeName, proc) {
			return true
		}
	}
	return false
}
```

### 5.9 `plugin/handlers/handlers_pid_status.go`

```go
// handlers_pid_status.go
package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.zabbix.com/sdk/errs"
)

// 标准映射表：英文key <-> 标准中文描述（仅保留能通过 /proc 采集的指标，rss/vsz 改为 vmrss/vmsize）
var pidstatDict = map[string]string{
	"%cpu":         "进程使用cpu的使用率",
	"minflt/s":     "从内存中加载数据时每秒出现的次要错误的数目",
	"majflt/s":     "从内存中加载数据时每秒出现的主要错误的数目",
	"vmrss":        "进程使用的物理内存",
	"vmsize":       "进程使用的虚拟内存",
	"%mem":         "进程使用物理内存的使用率",
	"kb_rd/s":      "进程每秒从磁盘读取的数据量",
	"kb_wr/s":      "进程每秒向磁盘写入的数据量",
	"threads":      "进程线程总数",
	"voluntary":    "每秒主动任务上下文切换的次数",
	"nonvoluntary": "每秒被动任务上下文切换的次数",
}

// 自动生成反向映射：标准中文描述 -> 英文key
var zhKeyMap = map[string]string{}

func init() {
	for k, v := range pidstatDict {
		zhKeyMap[v] = k
	}
	// 常用别名
	zhKeyMap["线程数"] = "threads"
	zhKeyMap["物理内存"] = "vmrss"
	zhKeyMap["虚拟内存"] = "vmsize"
	zhKeyMap["rss"] = "vmrss"
	zhKeyMap["vsz"] = "vmsize"
	zhKeyMap["cpu使用率"] = "%cpu"
	zhKeyMap["物理内存使用率"] = "%mem"
}

// 全局缓存结构
var (
	pidCache   = make(map[int]*procCache)
	cacheMutex sync.RWMutex
)

type procCache struct {
	lastTime       time.Time
	lastUtime      float64
	lastStime      float64
	lastMinflt     float64
	lastMajflt     float64
	lastReadBytes  float64
	lastWriteBytes float64
}

// 清理参数字符串，统一小写、去空格
func cleanMetricName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// GetPidStatus 主入口函数
func GetPidStatus(ctx context.Context, metricParams map[string]string) (interface{}, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[pid_status] panic recovered: %v\n", r)
		}
	}()
	metricName, metricExists := metricParams["metric"]
	pidStr, pidExists := metricParams["pid"]

	if !metricExists || !pidExists {
		return nil, errs.New("缺少 metric 或 pid 参数")
	}

	cleaned := cleanMetricName(metricName)
	// 1. 先查标准中文描述
	for k, v := range pidstatDict {
		if cleanMetricName(v) == cleaned {
			cleaned = k
			break
		}
	}
	// 2. 再查别名
	if mapped, ok := zhKeyMap[cleaned]; ok {
		cleaned = mapped
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("无效的 PID: %s", pidStr)
	}

	data, err := getPidData(pid)
	if err != nil {
		return nil, err
	}

	fmt.Printf("[GetPidStatus] cleaned: '%s', data: %+v\n", cleaned, data)
	for k, v := range data {
		if cleanMetricName(k) == cleaned {
			return v, nil
		}
	}
	fmt.Printf("[pid_status] 未匹配到指标: '%s' 字节: %v, 可用: %+v\n", metricName, []byte(metricName), data)
	return nil, fmt.Errorf("无效的指标名称: '%s'，可用指标: %v", metricName, data)
}

// 核心数据获取逻辑
func getPidData(pid int) (map[string]float64, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	now := time.Now()
	cache, exists := pidCache[pid]
	newCache := &procCache{lastTime: now}

	// 获取基础状态信息
	statusData, err := getStatus(pid)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[getPidData] statusData: %+v\n", statusData)

	// 获取CPU和缺页信息
	utime, stime, minflt, majflt, err := getCPUAndFaults(pid)
	if err != nil {
		return nil, err
	}

	// 获取IO数据
	readBytes, writeBytes, err := getIOStats(pid)
	if err != nil {
		return nil, err
	}

	// 初始化缓存数据
	newCache.lastUtime = utime
	newCache.lastStime = stime
	newCache.lastMinflt = minflt
	newCache.lastMajflt = majflt
	newCache.lastReadBytes = readBytes
	newCache.lastWriteBytes = writeBytes

	// 计算增量值
	var result = make(map[string]float64)
	if exists {
		timeDiff := now.Sub(cache.lastTime).Seconds()
		if timeDiff > 0 {
			result["%cpu"] = ((utime - cache.lastUtime) + (stime - cache.lastStime)) / timeDiff
			result["minflt/s"] = (minflt - cache.lastMinflt) / timeDiff
			result["majflt/s"] = (majflt - cache.lastMajflt) / timeDiff
			result["kb_rd/s"] = (readBytes - cache.lastReadBytes) / 1024 / timeDiff
			result["kb_wr/s"] = (writeBytes - cache.lastWriteBytes) / 1024 / timeDiff
		}
	} else {
		// 首次采集返回0值
		result["%cpu"] = 0
		result["minflt/s"] = 0
		result["majflt/s"] = 0
		result["kb_rd/s"] = 0
		result["kb_wr/s"] = 0
	}

	// 获取内存总量
	memTotal, _ := getMemTotal()
	if memTotal == 0 {
		memTotal = 1 // 防止除零
	}

	// 静态指标
	result["vmsize"] = parseStatusValue(statusData["vmsize"])
	result["vmrss"] = parseStatusValue(statusData["vmrss"])
	result["threads"] = parseStatusValue(statusData["threads"])
	result["voluntary"] = parseStatusValue(statusData["voluntary_ctxt_switches"])
	result["nonvoluntary"] = parseStatusValue(statusData["nonvoluntary_ctxt_switches"])
	result["%mem"] = result["vmrss"] / memTotal * 100

	fmt.Printf("[getPidData] result: %+v\n", result)

	// 更新缓存
	pidCache[pid] = newCache
	return result, nil
}

// 从/proc/[pid]/status获取状态信息
func getStatus(pid int) (map[string]string, error) {
	statusFile := fmt.Sprintf("/proc/%d/status", pid)
	file, err := os.Open(statusFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		data[key] = value
	}
	fmt.Printf("[getStatus] data: %+v\n", data)
	return data, scanner.Err()
}

// 从/proc/[pid]/stat获取CPU和缺页信息
func getCPUAndFaults(pid int) (float64, float64, float64, float64, error) {
	content, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, 0, 0, err
	}

	fields := strings.Fields(string(content))
	if len(fields) < 24 {
		return 0, 0, 0, 0, fmt.Errorf("invalid stat format")
	}

	utime, _ := strconv.ParseFloat(fields[13], 64)
	stime, _ := strconv.ParseFloat(fields[14], 64)
	minflt, _ := strconv.ParseFloat(fields[9], 64)
	majflt, _ := strconv.ParseFloat(fields[11], 64)

	return utime / 100, stime / 100, minflt, majflt, nil
}

// 从/proc/[pid]/io获取IO数据
func getIOStats(pid int) (float64, float64, error) {
	content, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/io", pid))
	if err != nil {
		return 0, 0, err
	}

	var readBytes, writeBytes float64
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "read_bytes:") {
			fmt.Sscanf(line, "read_bytes: %f", &readBytes)
		}
		if strings.HasPrefix(line, "write_bytes:") {
			fmt.Sscanf(line, "write_bytes: %f", &writeBytes)
		}
	}
	return readBytes, writeBytes, nil
}

// 获取系统内存总量
func getMemTotal() (float64, error) {
	content, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "MemTotal:") {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 {
				return strconv.ParseFloat(fields[1], 64)
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found")
}

// 辅助函数：转换状态值，支持带单位（如 '25592 kB'）
func parseStatusValue(value string) float64 {
	if value == "" {
		return 0
	}
	// 只取第一个数字部分
	fields := strings.Fields(value)
	f, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		fmt.Printf("[parseStatusValue] 转换失败: '%s'\n", value)
		return 0
	}
	return f
}

// 辅助函数：安全转换字符串
func safeParseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
```

### 5.10 `plugin/handlers/handlers_http_status.go`

```go
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
```

## 6. 编译与构建

```bash
cd ports

# 如需从零初始化模块（会删除已有依赖锁定）
# rm -rf go.mod go.sum zabbix-agent2-*

go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/ports
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
```

构建产物：`ports/zabbix-agent2-plugin-ports`。

## 7. 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-ports /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp ports.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-ports -V
./zabbix-agent2-plugin-ports -t ports.tcp.discovery
./zabbix-agent2-plugin-ports -t ports.udp.discovery
./zabbix-agent2-plugin-ports -t 'pid_status[%CPU,1234]'
./zabbix-agent2-plugin-ports -t 'pid_status[VmRSS,1234]'
./zabbix-agent2-plugin-ports -t 'http_status[80]'
```

## 8. 注意事项

- 插件仅支持 Linux，依赖 `/proc` 文件系统。
- 读取其他进程的 `/proc/<pid>/fd`、`/proc/<pid>/io` 通常需要 root；非 root 运行会漏掉部分进程或 I/O 指标。
- `pid_status` 的速率类指标（`%CPU`、`minflt/s`、`majflt/s`、`kB_rd/s`、`kB_wr/s`）依赖进程内缓存，**同一进程首次采集返回 0**，第二次及以后才有有效值。
- 端口发现默认并发数为 CPU 核心数的一半（向上取整，1-128），可通过环境变量 `PORT_DISCOVERY_MAX_CONCURRENCY` 覆盖。
- `ports.http.discovery` 会对每个 TCP 端口发起 HTTP 请求，单端口超时 2 秒，端口较多时耗时明显，建议适当增大插件 Timeout。
- `http_status` 未传 `Port` 参数时返回错误；请求失败返回 `0` 而不是错误。
- 配置项字段名是 `ExcludedProcesses`（不是 `ExcludeProcesses`），请确保 Agent 配置使用正确键名。
