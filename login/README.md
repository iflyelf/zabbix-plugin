# zabbix Login 插件

`Login` 插件统计当前主机在**当前分钟**内的登录失败次数，用于暴力破解检测和安全告警。

优先执行 `lastb | grep '<日> <时:分>' | wc -l`；若 `lastb` 不可用，则回退到在 `/var/log/secure`、
`/var/log/auth.log`、`/var/log/messages` 中匹配当前时间与 `Failed password` 的行数。

## 功能特性

- 统计当前分钟登录失败次数；
- `lastb` 失败时自动回退到日志文件匹配；
- 适合配置安全告警触发器。

## 监控项

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `login.failed` | 无 | 整数 | 当前分钟内登录失败次数 |

## 配置

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.Login.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.Login.Timeout` | 否 | 调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.Login.Sessions.*` | 否 | 命名会话（当前业务代码未使用） |
| `Plugins.Login.Default.*` | 否 | 默认会话（当前业务代码未使用） |

完整示例见同目录 `login.conf`。

## 代码结构

```text
login/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── login.conf                           # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
├── zabbix Login 插件编写.md             # 详细开发文档
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 指标注册与处理函数映射
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        ├── handlers_base.go             # WithJSONResponse 通用响应包装
        └── handlers_login_fail.go       # 登录失败次数统计
```

## 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-login /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp login.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-login -V
./zabbix-agent2-plugin-login -t login.failed
```

## 注意事项

- 该指标是**当前分钟**的失败次数，不是当天累计值。
- `lastb` 需要读取 `/var/log/btmp`，通常只有 root 可读；建议以 root 运行 Agent 2，或为运行用户授予日志读取权限。
- 回退逻辑依赖 `bash`、`find`、`grep`、`egrep` 等命令。
- 代码按“日 + 时:分”字符串匹配日志，跨分钟边界时可能漏计当分钟最早几秒的记录。

## 详细文档

完整源码说明、配置示例和开发细节见：[`zabbix Login 插件编写.md`](<./zabbix Login 插件编写.md>)

## 编译login

### 编译
```bash
rm -rf go.mod go.sum zabbix-agent2-*
go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/login
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
cp -rfp zabbix-agent2-* /usr/local/zabbix/sbin/zabbix-agent2-plugin/
```
