# zabbix Ports 插件

`Ports` 插件基于 Linux `/proc` 实现端口与进程监控，提供 TCP/UDP/HTTP 监听端口自动发现、
指定 PID 的进程性能指标，以及本地端口 HTTP 可访问性检查。

端口发现采用“先解析 `/proc/net/tcp*` 的 inode→端口映射，再反查 `/proc/<pid>/fd`”的方式，
并支持并发扫描与端口去重。

## 功能特性

- TCP/UDP/HTTP 监听端口自动发现；
- 进程 CPU、内存、I/O、缺页、上下文切换指标；
- 本地端口 HTTP 状态检查；
- 支持配置追加排除进程，并内置常见系统/监控进程排除列表。

## 监控项

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `ports.tcp.discovery` | 无 | JSON 数组字符串 | 发现 LISTEN 状态的 TCP 端口及进程 |
| `ports.udp.discovery` | 无 | JSON 数组字符串 | 发现 UDP 端口及进程 |
| `ports.http.discovery` | 无 | JSON 数组字符串 | 筛选能返回 HTTP 响应头的 TCP 端口 |
| `pid_status` | `metric`、`pid` | 数值字符串 | 指定进程性能指标 |
| `http_status` | `Port` | `1` / `0` | 本地端口 HTTP 检查 |

`pid_status` 常用 `metric`：`%CPU`、`%MEM`、`VmSize`、`VmRSS`、`Threads`、`minflt/s`、`majflt/s`、
`kB_rd/s`、`kB_wr/s`、`voluntary`、`nonvoluntary`，同时支持对应中文描述。

## 配置

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.Ports.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.Ports.Timeout` | 否 | 调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.Ports.ExcludedProcesses` | 否 | 追加排除进程，多个名称用英文逗号分隔 |
| `Plugins.Ports.Sessions.*` | 否 | 命名会话（当前业务代码未使用） |
| `Plugins.Ports.Default.*` | 否 | 默认会话（当前业务代码未使用） |

完整示例见同目录 `ports.conf`。

## 代码结构

```text
ports/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── ports.conf                           # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
├── zabbix Ports 插件编写.md             # 详细开发文档
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

## 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-ports /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp ports.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-ports -V
./zabbix-agent2-plugin-ports -t ports.tcp.discovery
./zabbix-agent2-plugin-ports -t ports.udp.discovery
./zabbix-agent2-plugin-ports -t 'pid_status[%CPU,1234]'
./zabbix-agent2-plugin-ports -t 'http_status[80]'
```

## 注意事项

- 仅支持 Linux，依赖 `/proc` 文件系统。
- 读取其他进程的 `/proc/<pid>/fd`、`/proc/<pid>/io` 通常需要 root，非 root 运行会漏掉部分进程或 I/O 指标。
- `pid_status` 的速率类指标（`%CPU`、`minflt/s`、`majflt/s`、`kB_rd/s`、`kB_wr/s`）依赖进程内缓存，**首次采集返回 0**。
- 端口发现默认并发数为 CPU 核心数的一半（1-128），可通过环境变量 `PORT_DISCOVERY_MAX_CONCURRENCY` 覆盖。
- `ports.http.discovery` 会对每个 TCP 端口发起 HTTP 请求，单端口超时 2 秒，端口较多时建议增大插件 Timeout。
- 配置键名为 `ExcludedProcesses`（不是 `ExcludeProcesses`）。

## 详细文档

完整源码说明、配置示例和开发细节见：[`zabbix Ports 插件编写.md`](<./zabbix Ports 插件编写.md>)

## 编译ports

### 编译
```bash
rm -rf go.mod go.sum zabbix-agent2-*
go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/ports
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
cp -rfp zabbix-agent2-* /usr/local/zabbix/sbin/zabbix-agent2-plugin/
```
