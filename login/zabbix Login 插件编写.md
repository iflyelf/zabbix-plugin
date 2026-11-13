# zabbix Login 插件编写

> 适用版本：Zabbix Agent 2 8.0 / golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3
> 代码目录：`login/`
> 最后更新：2026-11-13

> **SDK 版本说明**：本文档对应 `golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3`。
> 从该版本开始，`plugin/flag.HandleFlags()` 不再接收参数，返回 `(*Arguments, error)`；
> 入口程序通过 `flag.DecideActionFromFlags(args, &plugin.Impl, pluginInfo, nil)` 处理 `-h/-V/-t/-v`，
> 并使用 `errors.Is(err, errs.ErrExitGracefully)` 判断正常退出。旧版 7 参数 `HandleFlags` 写法已不适用。
>
> **构建要求**：Go 1.25.12 及以上（SDK 的 `go.mod` 要求）。当前插件 `go.mod` 声明 `go 1.27.4`，
> Go 工具链会自动下载并使用对应版本，首次构建需要可访问 `GOPROXY`。

## 1. 插件简介

`Login` 插件统计当前主机在**当前分钟**内的登录失败次数，用于暴力破解检测和安全告警。

实现方式：
1. 优先执行 `lastb | grep '<日> <时:分>' | wc -l` 读取失败登录记录；
2. 若 `lastb` 不可用，则回退到在 `/var/log/secure`、`/var/log/auth.log`、`/var/log/messages` 中匹配当前时间与 `Failed password` 的行数。

## 2. 代码结构

```text
login/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── login.conf                           # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 指标注册与处理函数映射
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        ├── handlers_base.go             # WithJSONResponse 通用响应包装
        └── handlers_login_fail.go       # 登录失败次数统计
```

## 3. 监控项与返回值

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `login.failed` | 无 | 整数 | 当前分钟内登录失败次数 |

返回值经 `handlers.WithJSONResponse` 包装后以字符串形式输出（例如 `0`、`3`）。

## 4. 配置文件

配置文件默认安装到 Zabbix Agent 2 的插件配置目录，需通过 `Include=` 指令被主配置加载。

### 4.1 配置项说明

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.Login.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.Login.Timeout` | 否 | 插件调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.Login.Sessions.*.Username` | 否 | 命名会话用户名（当前业务代码未使用） |
| `Plugins.Login.Sessions.*.Password` | 否 | 命名会话密码（当前业务代码未使用） |
| `Plugins.Login.Default.Username` | 否 | 默认会话用户名（当前业务代码未使用） |
| `Plugins.Login.Default.Password` | 否 | 默认会话密码（当前业务代码未使用） |

### 4.2 完整配置示例

```ini
### 配置项: Plugins.Login.System.Path
#   插件可执行文件的路径，指向外部插件的路径。

# 必选: 是
# 默认:
Plugins.Login.System.Path=/usr/local/zabbix/sbin/zabbix-agent2-plugin/zabbix-agent2-plugin-login

### 配置项: Plugins.Login.Timeout
#   插件调用地址时的默认超时时间设置，如果未指定超时时间，则使用全局超时时间。

# 必选: 否
# 范围: 1-30
# 默认:
Plugins.Login.Timeout=30

### 配置项: Plugins.Login.Sessions.*.Username
#   会话参数，指定的用户名，这些用户名在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.Login.Sessions.*.Username=

### 配置项: Plugins.Login.Sessions.*.Password
#   会话参数，指定的密码，这些密码在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.Login.Sessions.*.Password=

### 配置项: Plugins.Login.Default.Username
#   默认会话参数，指定的用户名，这些用户名在代码中硬编码。
#   如果没有指定其他用户名，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.Login.Default.Username=

### 配置项: Plugins.Login.Default.Password
#   默认会话参数，指定的密码，这些密码在代码中硬编码。
#   如果没有指定其他密码，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.Login.Default.Password=
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

	"golang.zabbix.com/plugin/login/plugin"  // 导入登录失败插件包
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
const Name = "Login"

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

### 5.5 `plugin/handlers/handlers_base.go`

```go
// 定义基础处理函数模块
package handlers

import (
        "context"            // 导入 context 包，用于传递上下文信息
        "encoding/json"      // 导入 encoding/json 包，用于处理 JSON 编码和解码
        "fmt"                // 导入 fmt 包，用于格式化输入输出
        "golang.zabbix.com/sdk/errs" // 导入 Zabbix SDK 的错误处理包，用于包装错误
)

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

### 5.6 `plugin/handlers/handlers_login_fail.go`

```go
// handlers_login_fail.go 定义登录失败信息查询的处理函数
package handlers

import (
    "context"                     // 提供上下文管理功能，便于控制操作的生命周期
    "fmt"                         // 用于格式化字符串输出
    "os/exec"                     // 用于执行外部命令
    "strconv"                     // 用于字符串与整数之间的转换
    "strings"                     // 用于字符串操作
    "time"                        // 提供时间操作，如获取当前时间
)

// LoginFailedAttemptsHandler 查询并返回登录失败尝试次数
func LoginFailedAttemptsHandler(ctx context.Context, params map[string]string) (any, error) {
    // 获取当前的时间（日期和小时:分钟）
    currentTime := time.Now()
    day := fmt.Sprintf("%02d", currentTime.Day()) // 获取当前日期并格式化为两位数
    hourMinute := currentTime.Format("15:04") // 格式化当前时间为"小时:分钟"

    // 尝试获取 lastb 登录失败次数
    lastbCount, err := getLastbFailedAttempts(day, hourMinute)
    if err != nil {
        // 如果 lastb 命令失败，尝试使用日志文件的方法
        logFileCount, logErr := getLogFileFailedAttempts(day, hourMinute)
        if logErr != nil {
            return 0, fmt.Errorf("获取登录失败次数时出错: %v, %v", err, logErr)
        }
        return logFileCount, nil
    }

    // 返回 lastb 的结果
    return lastbCount, nil
}

// getLastbFailedAttempts 使用 lastb 命令获取登录失败次数
func getLastbFailedAttempts(day, hourMinute string) (int, error) {
    cmd := exec.Command("bash", "-c", fmt.Sprintf("lastb | grep '%s %s' | wc -l", day, hourMinute))

    output, err := cmd.Output()
    if err != nil {
        return 0, fmt.Errorf("执行 lastb 命令失败: %v", err)
    }

    // 解析命令输出
    countStr := strings.TrimSpace(string(output))
    count, err := strconv.Atoi(countStr) // 将输出转换为整数
    if err != nil {
        return 0, fmt.Errorf("转换登录失败次数时出错: %v", err)
    }

    return count, nil // 返回登录失败次数
}

// getLogFileFailedAttempts 从日志文件中获取登录失败次数
func getLogFileFailedAttempts(day, hourMinute string) (int, error) {
    cmd := exec.Command("bash", "-c", fmt.Sprintf(
        "find /var/log/ -iname 'secure' -or -iname 'auth.log' -or -iname 'messages' | xargs grep '%s %s' | egrep 'Failed password' | wc -l", day, hourMinute))

    output, err := cmd.Output()
    if err != nil {
        return 0, fmt.Errorf("执行日志查找命令失败: %v", err)
    }

    // 解析命令输出
    countStr := strings.TrimSpace(string(output))
    count, err := strconv.Atoi(countStr) // 将输出转换为整数
    if err != nil {
        return 0, fmt.Errorf("转换登录失败次数时出错: %v", err)
    }

    return count, nil // 返回登录失败次数
}
```

## 6. 编译与构建

```bash
cd login

# 如需从零初始化模块（会删除已有依赖锁定）
# rm -rf go.mod go.sum zabbix-agent2-*

go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/login
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
```

构建产物：`login/zabbix-agent2-plugin-login`。

## 7. 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-login /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp login.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-login -V
./zabbix-agent2-plugin-login -t login.failed
```

## 8. 注意事项

- 该指标是**当前分钟**的失败次数，不是当天累计值。若需要累计趋势，请在 Zabbix 侧使用触发器/趋势或调整代码为累计统计。
- `lastb` 需要读取 `/var/log/btmp`，通常只有 root 可读；建议以 root 运行 Agent 2，或为运行用户授予日志读取权限。
- 回退逻辑依赖 `bash`、`find`、`grep`、`egrep` 等命令，极简系统可能缺少 `egrep`（现代 grep 中为兼容脚本）。
- 代码按“日 + 时:分”字符串匹配日志，跨分钟边界时可能漏计当分钟最早几秒的记录。
