# zabbix_plugin

[![文档一致性检查](https://github.com/iflyelf/zabbix-plugin/actions/workflows/docs-check.yml/badge.svg)](https://github.com/iflyelf/zabbix-plugin/actions/workflows/docs-check.yml)
[![插件编译检查](https://github.com/iflyelf/zabbix-plugin/actions/workflows/build.yml/badge.svg)](https://github.com/iflyelf/zabbix-plugin/actions/workflows/build.yml)
[![GitHub Release](https://img.shields.io/github/v/release/iflyelf/zabbix-plugin)](https://github.com/iflyelf/zabbix-plugin/releases/latest)

Zabbix Agent 2（8.0）可加载插件集合，包含网卡地址发现、conntrack 状态、硬件信息、登录失败、端口/进程和 TCP 连接状态监控。

## 插件列表

| 插件 | 目录 | 配置文件 | 主要监控项 | 文档 |
|---|---|---|---|---|
| Address | `address/` | `address.conf` | `address.discovery` | [README](address/README.md) / [详细文档](<address/zabbix Address 插件编写.md>) |
| ConntrackStatus | `conntrack/` | `conntrack.conf` | `conntrack.info`、`conntrack.number` | [README](conntrack/README.md) / [详细文档](<conntrack/zabbix ConntrackStatus 插件.md>) |
| Hardware | `hardware/` | `hardware.conf` | `hardware.*` 共 18 项 | [README](hardware/README.md) / [详细文档](<hardware/zabbix hardware插件编写.md>) |
| Login | `login/` | `login.conf` | `login.failed` | [README](login/README.md) / [详细文档](<login/zabbix Login 插件编写.md>) |
| Ports | `ports/` | `ports.conf` | `ports.*.discovery`、`pid_status`、`http_status` | [README](ports/README.md) / [详细文档](<ports/zabbix Ports 插件编写.md>) |
| TcpStatus | `tcpconnect/` | `tcpconnect.conf` | `tcp.connect.status` | [README](tcpconnect/README.md) / [详细文档](<tcpconnect/zabbix TcpStatus 插件编写.md>) |

## 环境要求

- Go 1.25.12 及以上（当前 `go.mod` 声明 `go 1.27.4`，Go 工具链会自动下载对应版本）
- Zabbix Agent 2 8.0
- 依赖 SDK：`golang.zabbix.com/sdk v1.2.2-0.20260904100819-caedd9e13ed3`

## 构建

各插件的完整构建命令见对应目录下的 `README.md`，基本流程一致：

```bash
cd address
go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/address
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
```

构建产物为 `zabbix-agent2-plugin-<插件名>`，部署到：

```text
/usr/local/zabbix/sbin/zabbix-agent2-plugin/
```

并将对应 `.conf` 放入 Agent 2 的插件配置目录后，通过 `Include=` 加载。

## 文档检查

`scripts/` 下提供文档一致性检查脚本：

```bash
# 统一执行所有检查
./scripts/check_all.sh

# 单独检查 README 是否覆盖全部监控项
python3 scripts/check_readme_metrics.py

# 单独检查 Markdown 代码围栏是否成对闭合
python3 scripts/check_markdown_fences.py
```

脚本失败时返回非 0 退出码，可直接用于 CI 或 Git pre-commit。

## 发布新版本

发布流程由 `.github/workflows/release.yml` 自动完成：

1. 修改各插件 `main.go` 中的版本号（`PLUGIN_VERSION_MAJOR/MINOR/PATCH`）；
2. 提交并推送到 `main`；
3. 创建并推送版本标签：

```bash
git tag -a v8.0.1 -m "v8.0.1"
git push origin v8.0.1
```

4. GitHub Actions 自动执行：
   - 构建 6 个插件的 Linux amd64 二进制；
   - 构建 Hardware 插件的 Windows amd64 二进制；
   - 生成 `SHA256SUMS`；
   - 创建 GitHub Release 并上传全部资产，自动标记为最新版本。

也可以在 Actions 页面手动触发“构建并发布 Release”，输入已存在的标签（例如 `v8.0.1`）。

## 目录结构

```text
zabbix_plugin/
├── address/          # 网卡地址发现插件
├── conntrack/        # conntrack 状态插件
├── hardware/         # 硬件信息插件
├── login/            # 登录失败统计插件
├── ports/            # 端口与进程监控插件
├── tcpconnect/       # TCP 连接状态插件
├── goip/             # 独立的 IP 查询小程序
└── scripts/          # 文档检查脚本
```
