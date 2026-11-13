# zabbix ConntrackStatus 插件编写

> 适用版本：Zabbix Agent 2 8.0 / golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3
> 代码目录：`conntrack/`
> 最后更新：2026-11-13

> **SDK 版本说明**：本文档对应 `golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3`。
> 从该版本开始，`plugin/flag.HandleFlags()` 不再接收参数，返回 `(*Arguments, error)`；
> 入口程序通过 `flag.DecideActionFromFlags(args, &plugin.Impl, pluginInfo, nil)` 处理 `-h/-V/-t/-v`，
> 并使用 `errors.Is(err, errs.ErrExitGracefully)` 判断正常退出。旧版 7 参数 `HandleFlags` 写法已不适用。
>
> **构建要求**：Go 1.25.12 及以上（SDK 的 `go.mod` 要求）。当前插件 `go.mod` 声明 `go 1.27.4`，
> Go 工具链会自动下载并使用对应版本，首次构建需要可访问 `GOPROXY`。

## 1. 插件简介

`ConntrackStatus` 插件读取 Linux netfilter conntrack 的 `/proc` 文件，提供连接数及容量水位监控。
适用于防火墙/NAT 网关场景，用于提前发现 conntrack 表即将耗尽的风险。

## 2. 代码结构

```text
conntrack/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── conntrack.conf                       # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 指标注册与键值到处理函数的映射
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        └── handlers_conntrack_status.go # conntrack 状态读取与判断
```

## 3. 监控项与返回值

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `conntrack.info` | 无 | JSON 整数 `1` / `0` | 连接数 **未超过** `nf_conntrack_max` 的 70% 返回 `1`；超过返回 `0` |
| `conntrack.number` | 无 | JSON 整数 | 当前 `nf_conntrack_count` 连接数 |

读取路径：
- 最大连接数：`/proc/sys/net/nf_conntrack_max`
- 当前连接数：`/proc/sys/net/netfilter/nf_conntrack_count`

## 4. 配置文件

配置文件默认安装到 Zabbix Agent 2 的插件配置目录，需通过 `Include=` 指令被主配置加载。

### 4.1 配置项说明

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.ConntrackStatus.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.ConntrackStatus.Timeout` | 否 | 插件调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.ConntrackStatus.Sessions.*.Username` | 否 | 命名会话用户名（当前业务代码未使用） |
| `Plugins.ConntrackStatus.Sessions.*.Password` | 否 | 命名会话密码（当前业务代码未使用） |
| `Plugins.ConntrackStatus.Default.Username` | 否 | 默认会话用户名（当前业务代码未使用） |
| `Plugins.ConntrackStatus.Default.Password` | 否 | 默认会话密码（当前业务代码未使用） |

### 4.2 完整配置示例

```ini
### 配置项: Plugins.ConntrackStatus.System.Path
#   插件可执行文件的路径，指向外部插件的路径。

# 必选: 是
# 默认:
Plugins.ConntrackStatus.System.Path=/usr/local/zabbix/sbin/zabbix-agent2-plugin/zabbix-agent2-plugin-conntrack

### 配置项: Plugins.ConntrackStatus.Timeout
#   插件调用地址时的默认超时时间设置，如果未指定超时时间，则使用全局超时时间。

# 必选: 否
# 范围: 1-30
# 默认:
Plugins.ConntrackStatus.Timeout=30

### 配置项: Plugins.ConntrackStatus.Sessions.*.Username
#   会话参数，指定的用户名，这些用户名在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.ConntrackStatus.Sessions.*.Username=

### 配置项: Plugins.ConntrackStatus.Sessions.*.Password
#   会话参数，指定的密码，这些密码在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.ConntrackStatus.Sessions.*.Password=

### 配置项: Plugins.ConntrackStatus.Default.Username
#   默认会话参数，指定的用户名，这些用户名在代码中硬编码。
#   如果没有指定其他用户名，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.ConntrackStatus.Default.Username=

### 配置项: Plugins.ConntrackStatus.Default.Password
#   默认会话参数，指定的密码，这些密码在代码中硬编码。
#   如果没有指定其他密码，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.ConntrackStatus.Default.Password=
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

	"golang.zabbix.com/plugin/conntrack/plugin" // 导入 conntrack 状态插件包
	"golang.zabbix.com/sdk/errs"                // 导入 Zabbix SDK 错误包
	sdkplugin "golang.zabbix.com/sdk/plugin"    // 导入 Zabbix SDK 插件包
	"golang.zabbix.com/sdk/plugin/container"    // 导入 Zabbix SDK 容器包
	"golang.zabbix.com/sdk/plugin/flag"         // 导入 Zabbix SDK 标志包
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
const Name = "ConntrackStatus"

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

### 5.5 `plugin/handlers/handlers_conntrack_status.go`

```go
// handlers_conntrack_status.go：实现 conntrack 状态的查询和处理逻辑
package handlers

import (
    "context"      // 上下文管理，用于控制任务生命周期
    "encoding/json" // 导入 json 序列化模块，用于将整数转换为 JSON 格式
    "fmt"          // 格式化输出
    "io/ioutil"    // 读文件工具
    "strconv"      // 字符串与整数的转换
    "strings"      // 字符串处理，用于去除空白字符
)

// 读取文件并解析为整数
func readIntFromFile(filePath string) (int, error) {
    data, err := ioutil.ReadFile(filePath) // 读取文件内容
    if err != nil {
        return 0, err // 返回错误
    }
    str := strings.TrimSpace(string(data)) // 去除多余空白
    value, err := strconv.Atoi(str) // 转换字符串为整数
    return value, err
}

// ConntrackInfoHandler 处理 conntrack.info 指标，返回阈值比较结果
func ConntrackInfoHandler(ctx context.Context, params map[string]string) (any, error) {
    nfConntrackMax, err := readIntFromFile("/proc/sys/net/nf_conntrack_max")
    if err != nil {
        return nil, fmt.Errorf("读取 nf_conntrack_max 出错: %v", err)
    }
    nfConntrackNumber, err := readIntFromFile("/proc/sys/net/netfilter/nf_conntrack_count")
    if err != nil {
        return nil, fmt.Errorf("读取 nf_conntrack_count 出错: %v", err)
    }
    threshold := float64(nfConntrackMax) * 0.7 // 设置阈值为最大值的70%
    result := 1
    if float64(nfConntrackNumber) > threshold {
        result = 0
    }
    jsonResult, _ := json.Marshal(result)
    return string(jsonResult), nil
}

// ConntrackNumberHandler 处理 conntrack.number 指标，返回当前连接数
func ConntrackNumberHandler(ctx context.Context, params map[string]string) (any, error) {
    nfConntrackNumber, err := readIntFromFile("/proc/sys/net/netfilter/nf_conntrack_count")
    if err != nil {
        return nil, fmt.Errorf("读取 nf_conntrack_count 出错: %v", err)
    }
    jsonResult, _ := json.Marshal(nfConntrackNumber)
    return string(jsonResult), nil
}
```

## 6. 编译与构建

```bash
cd conntrack

# 如需从零初始化模块（会删除已有依赖锁定）
# rm -rf go.mod go.sum zabbix-agent2-*

go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/conntrack
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
```

构建产物：`conntrack/zabbix-agent2-plugin-conntrack`。

## 7. 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-conntrack /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp conntrack.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-conntrack -V
./zabbix-agent2-plugin-conntrack -t conntrack.info
./zabbix-agent2-plugin-conntrack -t conntrack.number
```

## 8. 注意事项

- 依赖内核模块 `nf_conntrack`。若 `/proc/sys/net/netfilter/nf_conntrack_count` 不存在，请先加载模块（例如 `modprobe nf_conntrack`）。
- `conntrack.info` 是水位告警项：`1` 表示正常，`0` 表示已超过 70% 阈值，建议触发器对 `0` 告警。
- 70% 阈值为代码内固定值，如需调整请修改 `handlers_conntrack_status.go`。
- 读取 `/proc` 文件通常不需要 root，但容器内需确保 `/proc` 已正确挂载。
