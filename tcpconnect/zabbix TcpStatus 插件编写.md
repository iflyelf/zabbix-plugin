# zabbix TcpStatus 插件编写

> 适用版本：Zabbix Agent 2 8.0 / golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3
> 代码目录：`tcpconnect/`
> 最后更新：2026-11-13

> **SDK 版本说明**：本文档对应 `golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3`。
> 从该版本开始，`plugin/flag.HandleFlags()` 不再接收参数，返回 `(*Arguments, error)`；
> 入口程序通过 `flag.DecideActionFromFlags(args, &plugin.Impl, pluginInfo, nil)` 处理 `-h/-V/-t/-v`，
> 并使用 `errors.Is(err, errs.ErrExitGracefully)` 判断正常退出。旧版 7 参数 `HandleFlags` 写法已不适用。
>
> **构建要求**：Go 1.25.12 及以上（SDK 的 `go.mod` 要求）。当前插件 `go.mod` 声明 `go 1.27.4`，
> Go 工具链会自动下载并使用对应版本，首次构建需要可访问 `GOPROXY`。

## 1. 插件简介

`TcpStatus` 插件执行 `ss -tan` 并统计不同 TCP 连接状态的数量，用于连接数趋势、异常状态排查和容量评估。

## 2. 代码结构

```text
tcpconnect/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── tcpconnect.conf                      # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 指标注册与处理函数映射
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        └── handlers_tcp_status.go       # ss 命令解析与状态统计
```

## 3. 监控项与返回值

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `tcp.connect.status` | `status_name` | JSON 整数 | 指定 TCP 状态的连接数 |

`status_name` 支持的取值（大小写不敏感）：

| 取值 | 对应 `ss` 状态 |
|---|---|
| `listen` | LISTEN |
| `estab` | ESTAB |
| `syn-sent` | SYN-SENT |
| `syn-recv` | SYN-RECV |
| `fin-wait-1` | FIN-WAIT-1 |
| `fin-wait-2` | FIN-WAIT-2 |
| `time-wait` | TIME-WAIT |
| `close-wait` | CLOSE-WAIT |
| `last-ack` | LAST-ACK |
| `closing` | CLOSING |
| `unconn` | 未匹配到上述状态的行 |

使用示例：`tcp.connect.status[listen]`、`tcp.connect.status[estab]`。

## 4. 配置文件

配置文件默认安装到 Zabbix Agent 2 的插件配置目录，需通过 `Include=` 指令被主配置加载。

### 4.1 配置项说明

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.TcpStatus.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.TcpStatus.Timeout` | 否 | 插件调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.TcpStatus.Sessions.*.Username` | 否 | 命名会话用户名（当前业务代码未使用） |
| `Plugins.TcpStatus.Sessions.*.Password` | 否 | 命名会话密码（当前业务代码未使用） |
| `Plugins.TcpStatus.Default.Username` | 否 | 默认会话用户名（当前业务代码未使用） |
| `Plugins.TcpStatus.Default.Password` | 否 | 默认会话密码（当前业务代码未使用） |

### 4.2 完整配置示例

```ini
### 配置项: Plugins.TcpStatus.System.Path
#   插件可执行文件的路径，指向外部插件的路径。

# 必选: 是
# 默认:
# Plugins.TcpStatus.System.Path=/usr/local/zabbix/sbin/zabbix-agent2-plugin/zabbix-agent2-plugin-tcpconnect

### 配置项: Plugins.TcpStatus.Timeout
#   插件调用地址时的默认超时时间设置，如果未指定超时时间，则使用全局超时时间。

# 必选: 否
# 范围: 1-30
# 默认:
# Plugins.TcpStatus.Timeout=30

### 配置项: Plugins.TcpStatus.Sessions.*.Username
#   会话参数，指定的用户名，这些用户名在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.TcpStatus.Sessions.*.Username=

### 配置项: Plugins.TcpStatus.Sessions.*.Password
#   会话参数，指定的密码，这些密码在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.TcpStatus.Sessions.*.Password=

### 配置项: Plugins.TcpStatus.Default.Username
#   默认会话参数，指定的用户名，这些用户名在代码中硬编码。
#   如果没有指定其他用户名，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.TcpStatus.Default.Username=

### 配置项: Plugins.TcpStatus.Default.Password
#   默认会话参数，指定的密码，这些密码在代码中硬编码。
#   如果没有指定其他密码，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.TcpStatus.Default.Password=
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

	"golang.zabbix.com/plugin/tcpconnect/plugin" // 导入 TCP 连接状态插件包
	"golang.zabbix.com/sdk/errs"                 // 导入 Zabbix SDK 错误包
	sdkplugin "golang.zabbix.com/sdk/plugin"     // 导入 Zabbix SDK 插件包
	"golang.zabbix.com/sdk/plugin/container"     // 导入 Zabbix SDK 容器包
	"golang.zabbix.com/sdk/plugin/flag"          // 导入 Zabbix SDK 标志包
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
const Name = "TcpStatus"

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

    // 初始化指标集合
    p.metrics = metrics

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

### 5.5 `plugin/handlers/handlers_tcp_status.go`

```go
// handlers_tcp_status.go：实现 TCP 连接状态的查询和处理逻辑
package handlers

import (
    "context" // 上下文管理，用于控制任务生命周期
    "encoding/json" // 导入 json 序列化模块，用于将整数转换为 JSON 格式
    "fmt"     // 格式化输出
    "os/exec" // 执行系统命令
    "strings" // 字符串处理，用于解析命令输出
)

// 定义 TCP 状态结构体，用于存储各种连接状态的计数
type TcpStatus struct {
    Listen    int // LISTEN 状态的连接数（正在监听的端口）
    Estab     int // ESTABLISHED 状态的连接数（已建立的连接）
    SynSent   int // SYN-SENT 状态的连接数（发送同步请求）
    SynRecv   int // SYN-RECV 状态的连接数（接收同步请求）
    FinWait1  int // FIN-WAIT-1 状态的连接数（第一次关闭等待）
    FinWait2  int // FIN-WAIT-2 状态的连接数（第二次关闭等待）
    TimeWait  int // TIME-WAIT 状态的连接数（等待关闭）
    CloseWait int // CLOSE-WAIT 状态的连接数（等待对方关闭）
    LastAck   int // LAST-ACK 状态的连接数（最后确认）
    Closing   int // CLOSING 状态的连接数（正在关闭）
    Unconn    int // 未知状态的连接数（无法识别的状态）
}

// 获取当前系统的 TCP 连接状态并进行统计
func getTcpStatus() TcpStatus {
    tcpStatus := TcpStatus{} // 初始化 TCP 状态结构体

    // 使用 ss 命令获取 TCP 连接信息（ss 工具更适合于新系统）
    cmd := exec.Command("ss", "-tan")
    output, err := cmd.Output() // 执行命令并获取输出
    if err != nil {
        fmt.Println("获取 TCP 状态失败:", err) // 打印错误信息
        return tcpStatus // 返回空的状态结构体
    }

    // 将命令输出按行分割为字符串数组
    lines := strings.Split(string(output), "\n")

    // 遍历每一行，统计各个状态的连接数
    for _, line := range lines[1:] {
        switch {
        case strings.Contains(line, "LISTEN"):
            tcpStatus.Listen++ // 统计 LISTEN 状态
        case strings.Contains(line, "ESTAB"):
            tcpStatus.Estab++ // 统计 ESTABLISHED 状态
        case strings.Contains(line, "SYN-SENT"):
            tcpStatus.SynSent++ // 统计 SYN-SENT 状态
        case strings.Contains(line, "SYN-RECV"):
            tcpStatus.SynRecv++ // 统计 SYN-RECV 状态
        case strings.Contains(line, "FIN-WAIT-1"):
            tcpStatus.FinWait1++ // 统计 FIN-WAIT-1 状态
        case strings.Contains(line, "FIN-WAIT-2"):
            tcpStatus.FinWait2++ // 统计 FIN-WAIT-2 状态
        case strings.Contains(line, "TIME-WAIT"):
            tcpStatus.TimeWait++ // 统计 TIME-WAIT 状态
        case strings.Contains(line, "CLOSE-WAIT"):
            tcpStatus.CloseWait++ // 统计 CLOSE-WAIT 状态
        case strings.Contains(line, "LAST-ACK"):
            tcpStatus.LastAck++ // 统计 LAST-ACK 状态
        case strings.Contains(line, "CLOSING"):
            tcpStatus.Closing++ // 统计 CLOSING 状态
        default:
            tcpStatus.Unconn++ // 统计无法识别的状态
        }
    }
    return tcpStatus // 返回统计结果
}

// TcpConnectStatusHandler 处理指定的 TCP 连接状态，返回所请求状态的计数
// 参数：ctx 为上下文对象，params 为查询时的参数（如状态名称）
func TcpConnectStatusHandler(ctx context.Context, params map[string]string) (any, error) {
    // 获取当前系统的 TCP 状态统计
    // 从参数中提取 "status_name" 以确定要查询的状态
    statusKey, ok := params["status_name"]
    if !ok {                                    // 检查是否提供了状态名称
        return nil, fmt.Errorf("缺少状态名称")    // 如果没有，返回错误
    }

    tcpStatus := getTcpStatus()                 // 调用 getTcpStatus 获取所有 TCP 状态的当前计数

    // 定义输出的结果为int类型
    var result int

    switch strings.ToLower(statusKey) {  // 根据状态名称选择对应的计数
    case "listen":
        result = tcpStatus.Listen // 返回 LISTEN 状态的连接数
    case "estab":
        result = tcpStatus.Estab // 返回 ESTABLISHED 状态的连接数
    case "syn-sent":
        result = tcpStatus.SynSent // 返回 SYN-SENT 状态的连接数
    case "syn-recv":
        result = tcpStatus.SynRecv // 返回 SYN-RECV 状态的连接数
    case "fin-wait-1":
        result = tcpStatus.FinWait1 // 返回 FIN-WAIT-1 状态的连接数
    case "fin-wait-2":
        result = tcpStatus.FinWait2 // 返回 FIN-WAIT-2 状态的连接数
    case "time-wait":
        result = tcpStatus.TimeWait // 返回 TIME-WAIT 状态的连接数
    case "close-wait":
        result = tcpStatus.CloseWait // 返回 CLOSE-WAIT 状态的连接数
    case "last-ack":
        result = tcpStatus.LastAck // 返回 LAST-ACK 状态的连接数
    case "closing":
        result = tcpStatus.Closing // 返回 CLOSING 状态的连接数
    case "unconn":
        result = tcpStatus.Unconn // 返回无法识别的状态计数
    default:
        return nil, fmt.Errorf("无效的状态名称: %s", statusKey) // 如果状态名称不正确，返回错误
    }

    // 将整数结果转换为 JSON 格式字符串，以确保传输过程中保持整数格式
    jsonResult, err := json.Marshal(result)     // 使用 json.Marshal 将整数结果编码为 JSON 格式
    if err != nil {                             // 检查编码过程是否出错
        return nil, fmt.Errorf("JSON 序列化失败: %s", err.Error()) // 如果出错，返回错误信息
    }

    // 返回 JSON 字符串，确保传输时保持数值格式
    return string(jsonResult), nil
}
```

## 6. 编译与构建

```bash
cd tcpconnect

# 如需从零初始化模块（会删除已有依赖锁定）
# rm -rf go.mod go.sum zabbix-agent2-*

go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/tcpconnect
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
```

构建产物：`tcpconnect/zabbix-agent2-plugin-tcpconnect`。

## 7. 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-tcpconnect /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp tcpconnect.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-tcpconnect -V
./zabbix-agent2-plugin-tcpconnect -t 'tcp.connect.status[listen]'
./zabbix-agent2-plugin-tcpconnect -t 'tcp.connect.status[estab]'
./zabbix-agent2-plugin-tcpconnect -t 'tcp.connect.status[time-wait]'
```

## 8. 注意事项

- 依赖系统命令 `ss`（iproute2 包），容器/精简系统中请先安装。
- `status_name` 为必填参数，缺失或取值非法时监控项会返回错误。
- `unconn` 统计所有未匹配状态关键字的行，包含 `ss` 输出末尾的空行，实际值可能比预期多 1。
- 每次采集都会执行一次 `ss -tan`，连接数很大时有一定 CPU/内存开销，不建议设置过短的采集间隔。
