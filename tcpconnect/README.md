# zabbix TcpStatus 插件

`TcpStatus` 插件执行 `ss -tan` 并统计不同 TCP 连接状态的数量，用于连接数趋势、异常状态排查和容量评估。

## 功能特性

- 统计 LISTEN、ESTAB、TIME-WAIT 等 11 类 TCP 状态；
- 通过监控项参数选择状态，使用简单；
- 适合连接数和容量趋势监控。

## 监控项

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `tcp.connect.status` | `status_name` | JSON 整数 | 指定 TCP 状态的连接数 |

`status_name` 支持：`listen`、`estab`、`syn-sent`、`syn-recv`、`fin-wait-1`、`fin-wait-2`、
`time-wait`、`close-wait`、`last-ack`、`closing`、`unconn`（大小写不敏感）。

示例：`tcp.connect.status[listen]`、`tcp.connect.status[estab]`。

## 配置

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.TcpStatus.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.TcpStatus.Timeout` | 否 | 调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.TcpStatus.Sessions.*` | 否 | 命名会话（当前业务代码未使用） |
| `Plugins.TcpStatus.Default.*` | 否 | 默认会话（当前业务代码未使用） |

完整示例见同目录 `tcpconnect.conf`。

## 代码结构

```text
tcpconnect/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── tcpconnect.conf                      # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
├── zabbix TcpStatus 插件编写.md         # 详细开发文档
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 指标注册与处理函数映射
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        └── handlers_tcp_status.go       # ss 命令解析与状态统计
```

## 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-tcpconnect /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp tcpconnect.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-tcpconnect -V
./zabbix-agent2-plugin-tcpconnect -t 'tcp.connect.status[listen]'
./zabbix-agent2-plugin-tcpconnect -t 'tcp.connect.status[estab]'
```

## 注意事项

- 依赖系统命令 `ss`（iproute2 包），容器/精简系统中请先安装。
- `status_name` 为必填参数，缺失或取值非法时监控项返回错误。
- `unconn` 统计所有未匹配状态关键字的行，包含 `ss` 输出末尾空行，实际值可能比预期多 1。
- 每次采集都会执行一次 `ss -tan`，连接数很大时有一定开销，不建议设置过短的采集间隔。

## 详细文档

完整源码说明、配置示例和开发细节见：[`zabbix TcpStatus 插件编写.md`](<./zabbix TcpStatus 插件编写.md>)

## 编译tcpconnect

### 编译
```bash
rm -rf go.mod go.sum zabbix-agent2-*
go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/tcpconnect
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
cp -rfp zabbix-agent2-* /usr/local/zabbix/sbin/zabbix-agent2-plugin/
```
