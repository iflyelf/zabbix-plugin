# zabbix Address 插件编写

> 适用版本：Zabbix Agent 2 8.0 / golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3
> 代码目录：`address/`
> 最后更新：2026-11-13

> **SDK 版本说明**：本文档对应 `golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3`。
> 从该版本开始，`plugin/flag.HandleFlags()` 不再接收参数，返回 `(*Arguments, error)`；
> 入口程序通过 `flag.DecideActionFromFlags(args, &plugin.Impl, pluginInfo, nil)` 处理 `-h/-V/-t/-v`，
> 并使用 `errors.Is(err, errs.ErrExitGracefully)` 判断正常退出。旧版 7 参数 `HandleFlags` 写法已不适用。
>
> **构建要求**：Go 1.25.12 及以上（SDK 的 `go.mod` 要求）。当前插件 `go.mod` 声明 `go 1.27.4`，
> Go 工具链会自动下载并使用对应版本，首次构建需要可访问 `GOPROXY`。

## 1. 插件简介

`Address` 插件通过 Go 标准库 `net.Interfaces()` 枚举本机网卡，输出 Zabbix 低级别自动发现（LLD）所需 JSON，
用于自动创建按网卡/IP 的监控项原型。

数据过滤规则：
- 跳过 loopback 接口；
- 跳过未启用（down）的接口；
- 跳过不支持多播的接口；
- 跳过常见虚拟网卡前缀：`docker`、`br-`、`veth`、`virbr`、`vmnet`、`vboxnet`、`zt`、`tun`、`tap`、`lo`、`cni`、`flannel`、`ifb`；
- 仅收集 IPv4 地址；没有 IPv4 地址的接口不输出。

## 2. 代码结构

```text
address/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── address.conf                         # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 指标注册与键值到处理函数的映射
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        └── handlers_address_discovery.go # 网卡地址发现处理函数
```

## 3. 监控项与返回值

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `address.discovery` | 无 | JSON 数组字符串 | 自动发现物理网卡及其 IPv4 地址 |

返回示例：

```json
[{"{#InterfaceName}":"ens33","{#IPAddresses}":["10.0.88.88"]}]
```

Zabbix LLD 宏：`{#InterfaceName}`、`{#IPAddresses}`。

## 4. 配置文件

配置文件默认安装到 Zabbix Agent 2 的插件配置目录，需通过 `Include=` 指令被主配置加载。

### 4.1 配置项说明

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.Address.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.Address.Timeout` | 否 | 插件调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.Address.Sessions.*.Username` | 否 | 命名会话用户名（当前业务代码未使用） |
| `Plugins.Address.Sessions.*.Password` | 否 | 命名会话密码（当前业务代码未使用） |
| `Plugins.Address.Default.Username` | 否 | 默认会话用户名（当前业务代码未使用） |
| `Plugins.Address.Default.Password` | 否 | 默认会话密码（当前业务代码未使用） |

### 4.2 完整配置示例

```ini
### 配置项: Plugins.Address.System.Path
#   插件可执行文件的路径，指向外部插件的路径。

# 必选: 是
# 默认:
Plugins.Address.System.Path=/usr/local/zabbix/sbin/zabbix-agent2-plugin/zabbix-agent2-plugin-address

### 配置项: Plugins.Address.Timeout
#   插件调用地址时的默认超时时间设置，如果未指定超时时间，则使用全局超时时间。

# 必选: 否
# 范围: 1-30
# 默认:
Plugins.Address.Timeout=30

### 配置项: Plugins.Address.Sessions.*.Username
#   会话参数，指定的用户名，这些用户名在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.Address.Sessions.*.Username=

### 配置项: Plugins.Address.Sessions.*.Password
#   会话参数，指定的密码，这些密码在代码中硬编码。
#   "*" 应替换为会话名称。

# 必选: 否
# 默认:
# Plugins.Address.Sessions.*.Password=

### 配置项: Plugins.Address.Default.Username
#   默认会话参数，指定的用户名，这些用户名在代码中硬编码。
#   如果没有指定其他用户名，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.Address.Default.Username=

### 配置项: Plugins.Address.Default.Password
#   默认会话参数，指定的密码，这些密码在代码中硬编码。
#   如果没有指定其他密码，则使用该默认值。

# 必选: 否
# 默认: "Zabbix"
# Plugins.Address.Default.Password=
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

	"golang.zabbix.com/plugin/address/plugin" // 导入网卡地址插件包
	"golang.zabbix.com/sdk/errs"              // 导入 Zabbix SDK 错误包
	sdkplugin "golang.zabbix.com/sdk/plugin"  // 导入 Zabbix SDK 插件包
	"golang.zabbix.com/sdk/plugin/container"  // 导入 Zabbix SDK 容器包
	"golang.zabbix.com/sdk/plugin/flag"       // 导入 Zabbix SDK 标志包
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
// 定义所有地址监控指标及其参数设置
package plugin

import (
	"context"                                    // 用于管理插件中的上下文传递，便于控制执行流程和取消操作
	"log"                                        // 用于在控制台或日志文件中记录日志信息
	"golang.zabbix.com/plugin/address/plugin/handlers" // 引入地址信息的处理程序库，处理不同的监控项
	"golang.zabbix.com/sdk/metric"               // 引入 SDK 的指标模块，定义和管理监控指标
	"golang.zabbix.com/sdk/plugin"               // 引入插件管理模块，用于插件注册和初始化
)

// 定义地址监控项的键名，用于标识每个监控项
const (
	keyAddressDiscovery = "address.discovery" // 地址发现监控项
)

// 定义一个映射，将每个监控项的键与对应的处理函数关联，以便动态调用
var metricHandlers = map[string]func(context.Context, map[string]string) (any, error){
	keyAddressDiscovery: handlers.AddressDiscoveryHandler, // 地址发现处理函数
}

// 初始化每个监控项的指标集合，定义指标的名称、参数和是否聚合
var metrics = metric.MetricSet{
	keyAddressDiscovery: metric.New("地址发现信息", nil, false), // 地址发现指标，不需要参数
}

// init 函数在程序启动时自动运行，将所有定义的监控项注册到插件中
func init() {
	// 调用插件 SDK 的 RegisterMetrics 函数，将监控项集合注册到插件
	err := plugin.RegisterMetrics(&Impl, Name, metrics.List()...)
	if err != nil { // 检查注册过程中是否有错误
		log.Fatalf("无法注册指标: %v", err) // 如果有错误，记录并终止程序
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
const Name = "Address"

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

### 5.5 `plugin/handlers/handlers_address_discovery.go`

```go
// handlers_discovery.go
package handlers

import (
	"context"                                 // 用于插件上下文传递与控制
	"encoding/json"                           // 用于编码输出为 JSON 字符串
	"net"                                     // 提供系统网络接口查询能力
	"strings"                                 // 用于处理字符串匹配操作
	"golang.zabbix.com/sdk/errs"              // Zabbix 插件 SDK 的错误封装工具
)

// JSONOutput 定义 JSON 输出结构体，用于 Zabbix 自动发现格式
type JSONOutput struct {
	InterfaceName string   `json:"{#InterfaceName}"` // 网卡名称字段，Zabbix LLD 要求格式
	IPAddresses   []string `json:"{#IPAddresses}"`   // 网卡对应的 IPv4 地址列表
}

// isVirtualInterface 判断接口名称是否属于常见的虚拟网卡
func isVirtualInterface(name string) bool {
	// 常见虚拟网卡的前缀列表，可按需扩展
	virtualPrefixes := []string{
		"docker",   // Docker 虚拟网卡，如 docker0
		"br-",      // 桥接网卡，如 br-xxxxx
		"veth",     // 虚拟 Ethernet，如 vethxxxx
		"virbr",    // libvirt 虚拟网桥，如 virbr0
		"vmnet",    // VMware 网络接口，如 vmnet1
		"vboxnet",  // VirtualBox 网卡，如 vboxnet0
		"zt",       // ZeroTier 网络
		"tun",      // VPN 隧道接口，如 tun0
		"tap",      // 虚拟 TAP 接口
		"lo",       // loopback（虽然已经过滤，此处冗余保险）
		"cni",      // 容器网络接口（Kubernetes）
		"flannel",  // Kubernetes flannel 网络插件
		"ifb",      // 中间网卡（中断过滤）
	}
	// 遍历虚拟前缀，如果网卡名称以其开头则判定为虚拟接口
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// AddressDiscoveryHandler 是用于 Zabbix 自动发现的处理函数
func AddressDiscoveryHandler(ctx context.Context, _ map[string]string) (any, error) {
	// 获取本地所有网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, errs.Wrap(err, "获取网卡接口信息失败")
	}

	// 定义用于存放最终输出结果的切片
	var result []JSONOutput

	// 遍历所有网络接口
	for _, iface := range interfaces {
		// 条件 1：跳过 loopback（回环）接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// 条件 2：跳过未启用（down）的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		// 条件 3：跳过不支持多播的接口（大多物理接口都支持）
		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		// 条件 4：跳过被认为是虚拟网卡的接口
		if isVirtualInterface(iface.Name) {
			continue
		}

		// 获取该网卡的所有地址
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, errs.Wrapf(err, "获取网卡地址失败: %s", iface.Name)
		}

		// 定义 IP 地址列表（只收集 IPv4 地址）
		var ipAddresses []string
		for _, addr := range addrs {
			// 类型断言：是否为 IP 网络地址
			if ipnet, ok := addr.(*net.IPNet); ok {
				// 判断是否为 IPv4 地址
				if ipnet.IP.To4() != nil {
					ipAddresses = append(ipAddresses, ipnet.IP.String())
				}
			}
		}

		// 如果该网卡至少有一个 IPv4 地址，则添加到结果列表中
		if len(ipAddresses) > 0 {
			result = append(result, JSONOutput{
				InterfaceName: iface.Name,
				IPAddresses:   ipAddresses,
			})
		}
	}

	// 将结果结构转换为 JSON 字符串
	jsonResult, err := json.Marshal(result)
	if err != nil {
		return nil, errs.Wrap(err, "转换结果为 JSON 失败")
	}

	// 最终返回 JSON 字符串（类型为字符串）
	return string(jsonResult), nil
}
```

## 6. 编译与构建

```bash
cd address

# 如需从零初始化模块（会删除已有依赖锁定）
# rm -rf go.mod go.sum zabbix-agent2-*

go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/address
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
```

构建产物：`address/zabbix-agent2-plugin-address`。

## 7. 部署与测试

```bash
# 部署可执行文件
cp -rfp zabbix-agent2-plugin-address /usr/local/zabbix/sbin/zabbix-agent2-plugin/

# 部署插件配置（按实际 Zabbix 配置目录调整）
cp -rfp address.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

# 本地验证
./zabbix-agent2-plugin-address -V
./zabbix-agent2-plugin-address -t address.discovery
```

## 8. 注意事项

- 只输出 IPv4 地址，IPv6 地址不会出现在 `{#IPAddresses}` 中。
- 虚拟网卡前缀写死在 `handlers_address_discovery.go` 的 `isVirtualInterface()` 中，如需发现 VLAN、bond、团队网卡等，请根据实际命名调整前缀列表。
- 插件无外部命令依赖，资源开销很小，适合较高频率执行发现。
