# zabbix Hardware 插件

`Hardware` 插件采集服务器硬件与操作系统基础信息，数据来源：
- `github.com/jaypipes/ghw`：系统、主板、物理磁盘信息；
- `github.com/shirou/gopsutil`：CPU、内存、磁盘使用率、操作系统信息。

适用于资产信息采集、容量趋势和硬件变更发现。

## 功能特性

- 系统、主板、CPU、内存、磁盘、操作系统共 18 个指标；
- 容量类指标自动选择 `KB/MB/GB/TB` 单位；
- 支持 Linux 与 Windows 构建。

## 监控项

| 监控项键值 | 参数 | 返回值 | 说明 |
|---|---|---|---|
| `hardware.system.manufacturer` | 无 | 字符串 | 系统制造商 |
| `hardware.system.productName` | 无 | 字符串 | 系统产品名称 |
| `hardware.system.serialNumber` | 无 | 字符串 | 系统序列号 |
| `hardware.system.uuid` | 无 | 字符串 | 系统 UUID |
| `hardware.baseboard.manufacturer` | 无 | 字符串 | 主板制造商 |
| `hardware.baseboard.productName` | 无 | 字符串 | 主板产品名称 |
| `hardware.baseboard.version` | 无 | 字符串 | 主板版本 |
| `hardware.baseboard.serialNumber` | 无 | 字符串 | 主板序列号 |
| `hardware.cpu.model` | 无 | 字符串 | CPU 型号 |
| `hardware.cpu.usage` | `duration`（可选，默认 `1s`） | 百分比字符串 | CPU 使用率 |
| `hardware.cpu.cores` | 无 | 字符串整数 | CPU 逻辑核心数 |
| `hardware.memory.usage` | 无 | 百分比字符串 | 内存使用率 |
| `hardware.memory.total` | 无 | 容量字符串 | 内存总容量，如 `19 GB` |
| `hardware.disk.capacity` | `path`（当前未参与计算） | 容量字符串 | 物理磁盘总容量 |
| `hardware.disk.usage` | `path`（当前未参与计算） | 百分比字符串 | 所有有效分区聚合使用率 |
| `hardware.os.name` | 无 | 字符串 | 操作系统名称 |
| `hardware.os.type` | 无 | 字符串 | 操作系统类型 |
| `hardware.os.version` | 无 | 字符串 | 操作系统版本 |

完整指标说明见同目录开发文档。

## 配置

| 配置项 | 必选 | 说明 |
|---|---|---|
| `Plugins.Hardware.System.Path` | 是 | 插件可执行文件路径 |
| `Plugins.Hardware.Timeout` | 否 | 调用超时，范围 1-30 秒；未设置时继承全局 Timeout |
| `Plugins.Hardware.Sessions.*` | 否 | 命名会话（当前业务代码未使用） |
| `Plugins.Hardware.Default.*` | 否 | 默认会话（当前业务代码未使用） |

完整示例见同目录 `hardware.conf`。

## 代码结构

```text
hardware/
├── main.go                              # 程序入口：解析参数、注册插件、启动 RPC 服务
├── hardware.conf                        # Zabbix Agent 2 插件配置示例
├── Makefile                             # 编译脚本
├── go.mod / go.sum                      # Go 模块与依赖锁定
├── zabbix hardware插件编写.md           # 详细开发文档
└── plugin/
    ├── config.go                        # 插件配置结构、Configure/Validate
    ├── metrics.go                       # 18 个硬件指标注册
    ├── plugin.go                        # 插件生命周期与 Export 分发
    └── handlers/
        └── handlers_hardware.go         # 硬件/系统信息采集处理函数
```

## 部署与测试

```bash
cp -rfp zabbix-agent2-plugin-hardware /usr/local/zabbix/sbin/zabbix-agent2-plugin/
cp -rfp hardware.conf /usr/local/zabbix/etc/zabbix_agent2.d/plugins.d/

./zabbix-agent2-plugin-hardware -V
./zabbix-agent2-plugin-hardware -t hardware.system.manufacturer
./zabbix-agent2-plugin-hardware -t 'hardware.cpu.usage[2s]'
./zabbix-agent2-plugin-hardware -t hardware.memory.total
```

## 注意事项

- `hardware.cpu.usage` 默认阻塞采样 1 秒，可通过参数调整；采样时间过长时需同步调大插件 Timeout。
- `hardware.disk.capacity` 与 `hardware.disk.usage` 虽然声明了 `path` 参数，但当前实现未使用：容量统计所有物理磁盘，使用率聚合所有有效分区。
- 部分硬件信息在虚拟机/容器中可能为空或返回 `"未知"`；Linux 下读取 DMI/SMBIOS 通常需要 root。
- Windows 构建时可使用 Makefile 中的 `windres` 资源，使 `-V` 输出文件版本信息。

## 详细文档

完整源码说明、配置示例和开发细节见：[`zabbix hardware插件编写.md`](<./zabbix hardware插件编写.md>)

## 编译hardware

### 编译
```bash
rm -rf go.mod go.sum zabbix-agent2-*
go env -w GO111MODULE=auto
go mod init golang.zabbix.com/plugin/hardware
GOPROXY=https://goproxy.cn go get golang.zabbix.com/sdk@v1.2.2-0.20260904100819-caedd9e13ed3
go mod tidy
make -j$(($(nproc)+1))
cp -rfp zabbix-agent2-* /usr/local/zabbix/sbin/zabbix-agent2-plugin/
CGO_ENABLED=0 GOOS="windows" GOARCH="amd64" go build -o "/data/hardware/zabbix-agent2-plugin-hardware.exe"
```
