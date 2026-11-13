# zabbix ConntrackStatus 插件

`ConntrackStatus` 插件读取 Linux netfilter conntrack 的 `/proc` 文件，提供当前连接数和容量水位监控，
适用于防火墙/NAT 网关场景，用于提前发现 conntrack 表即将耗尽的风险。

## 功能特性

- 读取 `/proc/sys/net/nf_conntrack_max` 与 `/proc/sys/net/netfilter/nf_conntrack_count`；
- 提供连接数绝对值指标；
- 提供 70% 水位判断指标，便于直接配置触发器。

## 监控项

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `conntrack.info` | 无 | JSON 整数 `1` / `0` | 未超过最大连接数 70% 返回 `1`，超过返回 `0` |
| `conntrack.number` | 无 | JSON 整数 | 当前 conntrack 连接数 |

## 配置

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.ConntrackStatus.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.ConntrackStatus.Timeout` | 否 | 调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.ConntrackStatus.Sessions.*` | 否 | 命名会话（当前业务代码未使用） |
| `Plugins.ConntrackStatus.Default.*` | 否 | 默认会话（当前业务代码未使用） |

完整示例见同目录 `conntrack.conf`。

## 代码结构

```text
conntrack/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── conntrack.conf                       # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
├── zabbix ConntrackStatus 插件.md       # 详细开发文档
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 指标注册与处理函数映射
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        └── handlers_conntrack_status.go # conntrack 状态读取与判断
```

## 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-conntrack /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp conntrack.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-conntrack -V
./zabbix-agent2-plugin-conntrack -t conntrack.info
./zabbix-agent2-plugin-conntrack -t conntrack.number
```

## 注意事项

- 依赖内核模块 `nf_conntrack`，若 `/proc/sys/net/netfilter/nf_conntrack_count` 不存在，请先加载模块。
- `conntrack.info` 返回 `1` 表示正常、`0` 表示已超过 70% 阈值，建议触发器对 `0` 告警。
- 70% 阈值为代码内固定值，如需调整请修改 `handlers_conntrack_status.go`。

## 详细文档

完整源码说明、配置示例和开发细节见：[`zabbix ConntrackStatus 插件.md`](<./zabbix ConntrackStatus 插件.md>)

## 编译conntrack

### 编译
```bash
rm -rf go.mod go.sum zabbix-agent2-*
go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/conntrack
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
cp -rfp zabbix-agent2-* /usr/local/zabbix/sbin/zabbix-agent2-plugin/
```
