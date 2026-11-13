# zabbix Address 插件

`Address` 插件通过 Go 标准库 `net.Interfaces()` 枚举本机网卡，输出 Zabbix 低级别自动发现（LLD）所需的 JSON，
用于自动创建按网卡/IP 的监控项原型。插件只收集 IPv4 地址，并默认过滤回环、down、非多播以及常见虚拟网卡。

## 功能特性

- 自动发现物理网卡及其 IPv4 地址；
- 过滤 loopback、未启用、不支持多播的接口；
- 过滤常见虚拟网卡前缀：`docker`、`br-`、`veth`、`virbr`、`vmnet`、`vboxnet`、`zt`、`tun`、`tap`、`lo`、`cni`、`flannel`、`ifb`；
- 输出标准 Zabbix LLD JSON，宏为 `{#InterfaceName}`、`{#IPAddresses}`。

## 监控项

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `address.discovery` | 无 | JSON 数组字符串 | 自动发现网卡及 IPv4 地址 |

## 配置

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.Address.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.Address.Timeout` | 否 | 调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.Address.Sessions.*` | 否 | 命名会话（当前业务代码未使用） |
| `Plugins.Address.Default.*` | 否 | 默认会话（当前业务代码未使用） |

完整示例见同目录 `address.conf`。

## 代码结构

```text
address/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── address.conf                         # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
├── zabbix Address 插件编写.md           # 详细开发文档
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 指标注册与处理函数映射
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        └── handlers_address_discovery.go # 网卡地址发现处理函数
```

## 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-address /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp address.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-address -V
./zabbix-agent2-plugin-address -t address.discovery
```

## 注意事项

- 仅支持 IPv4，IPv6 地址不会出现在 `{#IPAddresses}` 中。
- 虚拟网卡前缀写死在 `handlers_address_discovery.go` 的 `isVirtualInterface()` 中，可按实际环境调整。
- 无外部命令依赖，资源开销小，适合较高频率执行发现。

## 详细文档

完整源码说明、配置示例和开发细节见：[`zabbix Address 插件编写.md`](<./zabbix Address 插件编写.md>)

## 编译address

### 编译
```bash
rm -rf go.mod go.sum zabbix-agent2-*
go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/address
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
cp -rfp zabbix-agent2-* /usr/local/zabbix/sbin/zabbix-agent2-plugin/
```
