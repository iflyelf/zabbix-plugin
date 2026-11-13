// 定义硬件信息查询的处理函数
package handlers

import (
	"context" // 提供上下文管理功能，便于控制操作的生命周期
	"fmt"     // 用于格式化字符串输出
	"math"    // 引入 math 包以便进行四舍五入
	"os"      // 提供操作系统功能，如获取命令行参数和设置环境变量
	"os/exec" // 提供执行系统命令的功能
	"strconv" // 提供字符串转换功能
	"strings" // 提供字符串操作功能
	"time"    // 提供时间操作，如设置采样时间间隔

	"github.com/jaypipes/ghw"         // 硬件信息库，用于获取系统硬件信息
	"github.com/shirou/gopsutil/cpu"  // 系统信息库，用于获取 CPU 相关信息
	"github.com/shirou/gopsutil/disk" // 系统信息库，用于获取磁盘相关信息
	"github.com/shirou/gopsutil/host" // 系统信息库，用于获取操作系统相关信息
	"github.com/shirou/gopsutil/mem"  // 系统信息库，用于获取内存相关信息
)

// init 函数在程序加载时执行，设置环境变量以禁用 ghw 库的警告信息
func init() {
	os.Setenv("GHW_DISABLE_WARNINGS", "1") // 禁用 ghw 库的警告信息输出
}

// convertSize 自动转换单位的函数
// 输入参数：size（以字节为单位的容量值）
// 返回值：转换后的容量值（float64）和单位（string）
func convertSize(size float64) (float64, string) {
	var unit string           // 用于存储单位（如 TB、GB、MB、KB）
	var convertedSize float64 // 用于存储转换后的容量值

	// 根据容量大小选择合适的单位
	switch {
	case size >= 1024*1024*1024*1024: // 如果容量大于等于 1 TB
		convertedSize = size / (1024 * 1024 * 1024 * 1024) // 转换为 TB
		unit = "TB"
	case size >= 1024*1024*1024: // 如果容量大于等于 1 GB
		convertedSize = size / (1024 * 1024 * 1024) // 转换为 GB
		unit = "GB"
	case size >= 1024*1024: // 如果容量大于等于 1 MB
		convertedSize = size / (1024 * 1024) // 转换为 MB
		unit = "MB"
	default: // 如果容量小于 1 MB，使用 KB
		convertedSize = size / 1024 // 转换为 KB
		unit = "KB"
	}

	return convertedSize, unit // 返回转换后的容量值和单位
}

// 系统制造商处理器，返回系统制造商信息
func SystemManufacturerHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Product() // 使用 ghw 库获取系统产品信息
	if err != nil {
		return "未知", err // 若发生错误，返回“未知”和错误信息
	}
	return info.Vendor, nil // 返回系统制造商信息
}

// 系统产品名称处理器，返回系统的产品名称
func SystemProductNameHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Product() // 获取系统产品名称信息
	if err != nil {
		return "未知", err // 返回错误时显示“未知”并附带错误信息
	}
	return info.Name, nil // 返回系统产品名称
}

// 系统序列号处理器，返回系统序列号
func SystemSerialNumberHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Product() // 获取系统序列号信息
	if err != nil {
		return "未知", err // 发生错误则返回“未知”和错误信息
	}
	return info.SerialNumber, nil // 返回系统序列号
}

// 系统 UUID 处理器，返回系统的 UUID
func SystemUUIDHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Product() // 获取系统 UUID 信息
	if err != nil {
		return "未知", err // 返回“未知”错误信息
	}
	return info.UUID, nil // 返回系统 UUID
}

// 主板制造商处理器，返回主板制造商信息
func BaseboardManufacturerHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Baseboard() // 获取主板制造商信息
	if err != nil {
		return "未知", err // 错误处理
	}
	return info.Vendor, nil // 返回主板制造商
}

// 主板产品名称处理器，返回主板产品名称
func BaseboardProductNameHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Baseboard() // 获取主板产品名称信息
	if err != nil {
		return "未知", err // 返回错误信息
	}
	return info.Product, nil // 返回主板产品名称
}

// 主板版本处理器，返回主板版本
func BaseboardVersionHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Baseboard() // 获取主板版本信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.Version, nil // 返回主板版本
}

// 主板序列号处理器，返回主板序列号
func BaseboardSerialNumberHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := ghw.Baseboard() // 获取主板序列号信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.SerialNumber, nil // 返回主板序列号
}

// CPU 型号处理器，返回 CPU 型号信息
func CPUModelHandler(ctx context.Context, params map[string]string) (any, error) {
	cpuInfo, err := cpu.Info() // 获取 CPU 信息
	if err != nil || len(cpuInfo) == 0 {
		return "未知", err // 错误时返回“未知”
	}
	return cpuInfo[0].ModelName, nil // 返回第一个 CPU 的型号
}

// CPU 使用率处理器，返回 CPU 使用率
func CPUUsageHandler(ctx context.Context, params map[string]string) (any, error) {
	sampleDuration := 1 * time.Second           // 设置默认采样时间为 1 秒
	if duration, ok := params["duration"]; ok { // 检查是否有自定义采样时间
		if d, err := time.ParseDuration(duration); err == nil {
			sampleDuration = d // 使用用户指定的采样时间
		}
	}
	usage, err := cpu.Percent(sampleDuration, false) // 获取 CPU 使用率
	if err != nil || len(usage) == 0 {
		return "未知", err // 错误时返回“未知”
	}
	return fmt.Sprintf("%.2f", usage[0]), nil // 格式化使用率并返回
}

// CPU 核心数处理器，返回 CPU 核心数量
func CPUCoresHandler(ctx context.Context, params map[string]string) (any, error) {
	cpuInfo, err := cpu.Info() // 获取 CPU 信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return fmt.Sprintf("%d", len(cpuInfo)), nil // 返回 CPU 核心数
}

// 内存使用率处理器，返回内存使用率
func MemoryUsageHandler(ctx context.Context, params map[string]string) (any, error) {
	memInfo, err := mem.VirtualMemory() // 获取内存使用率信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return fmt.Sprintf("%.2f", memInfo.UsedPercent), nil // 格式化内存使用率并返回
}

// MemoryTotalHandler 返回内存总大小并自动转换单位
// 参数：
// - ctx：上下文，用于控制命令执行的超时或取消
// - params：包含路径等信息的参数（此处未用到路径）
// 返回值：格式化后的内存大小信息（如 "16 GB"）和错误信息（如果有）
func MemoryTotalHandler(ctx context.Context, params map[string]string) (any, error) {
	// 获取系统内存信息
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return "未知", err // 如果获取失败，返回“未知”及错误信息
	}

	// 获取内存总大小（以字节为单位）
	totalMemory := float64(memInfo.Total)

	// 使用 convertSize 函数转换内存大小
	convertedSize, unit := convertSize(totalMemory)

	// 四舍五入并转为整数
	roundedSize := int(math.Round(convertedSize))

	// 格式化结果为整数并附加单位
	return fmt.Sprintf("%d %s", roundedSize, unit), nil
}

// DiskCapacityHandler 返回磁盘总容量并自动转换单位
// 参数：
// - ctx：上下文，用于控制命令执行
// - params：包含路径等信息的参数（此处未用到路径）
// 返回值：格式化后的容量信息（如 "500 GB"）和错误信息（如果有）
func DiskCapacityHandler(ctx context.Context, params map[string]string) (any, error) {
	// 首先尝试使用 ghw 库获取物理磁盘信息
	block, err := ghw.Block()
	if err != nil {
		return "未知", err
	}

	var total uint64 = 0
	var diskCount int = 0
	processedDevices := make(map[string]bool) // 用于跟踪已处理的设备

	// 只计算物理磁盘的容量，不包括分区
	for _, disk := range block.Disks {
		// 跳过 CD-ROM 等只读设备
		if disk.DriveType.String() == "cdrom" {
			continue
		}
		// 跳过虚拟设备，但保留 RBD 设备
		if disk.DriveType.String() == "loop" && !strings.Contains(disk.Name, "rbd") {
			continue
		}
		// 跳过 LVM 设备（dm-* 设备）
		if strings.HasPrefix(disk.Name, "dm-") {
			continue
		}
		// 跳过 CD-ROM 设备（sr* 设备）
		if strings.HasPrefix(disk.Name, "sr") {
			continue
		}
		// 跳过已处理的设备
		if processedDevices[disk.Name] {
			continue
		}

		total += disk.SizeBytes
		diskCount++
		processedDevices[disk.Name] = true
	}

	// 如果检测到的磁盘数量太少（少于1个），可能 ghw 库没有正确识别所有磁盘
	// 在这种情况下，我们使用备选方案（仅限 Linux）
	if diskCount < 1 {
		// 检查操作系统类型
		hostInfo, err := host.Info()
		if err == nil {
			switch hostInfo.PlatformFamily {
			case "linux":
				// 在 Linux 上使用 lsblk 命令作为备选方案，但过滤掉 LVM 设备和 CD-ROM
				cmd := exec.Command("lsblk", "-b", "-d", "-o", "SIZE,NAME", "--noheadings")
				output, err := cmd.Output()
				if err == nil {
					lines := strings.Split(strings.TrimSpace(string(output)), "\n")
					var lsblkTotal uint64 = 0

					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line != "" {
							parts := strings.Fields(line)
							if len(parts) >= 2 {
								deviceName := parts[1]
								// 跳过 LVM 设备
								if strings.HasPrefix(deviceName, "dm-") {
									continue
								}
								// 跳过 CD-ROM 设备
								if strings.HasPrefix(deviceName, "sr") {
									continue
								}
								if size, err := strconv.ParseUint(parts[0], 10, 64); err == nil {
									lsblkTotal += size
								}
							}
						}
					}
					// 如果 lsblk 的结果更大，使用 lsblk 的结果
					if lsblkTotal > total {
						total = lsblkTotal
					}
				}
			case "windows":
				// 在 Windows 上使用 gopsutil 的磁盘信息作为备选方案
				partitions, err := disk.Partitions(false)
				if err == nil {
					var winTotal uint64 = 0
					processedDevices := make(map[string]bool)
					for _, p := range partitions {
						if processedDevices[p.Device] {
							continue
						}
						usage, err := disk.Usage(p.Mountpoint)
						if err == nil {
							winTotal += usage.Total
							processedDevices[p.Device] = true
						}
					}
					// 如果 Windows 方法的结果更大，使用 Windows 的结果
					if winTotal > total {
						total = winTotal
					}
				}
			}
		}
	}

	convertedSize, unit := convertSize(float64(total))
	roundedSize := int(math.Round(convertedSize))
	return fmt.Sprintf("%d %s", roundedSize, unit), nil
}

// 磁盘使用率处理器，返回指定路径的磁盘使用率
func DiskUsageHandler(ctx context.Context, params map[string]string) (any, error) {
	// 获取所有挂载点
	partitions, err := disk.Partitions(false)
	if err != nil {
		return "未知", err
	}

	var total uint64 = 0
	var used uint64 = 0
	processedDevices := make(map[string]bool) // 用于跟踪已处理的设备

	// 遍历所有分区，但只计算每个物理设备一次
	for _, p := range partitions {
		// 跳过已处理的设备
		if processedDevices[p.Device] {
			continue
		}

		usage, err := disk.Usage(p.Mountpoint)
		if err == nil {
			total += usage.Total
			used += usage.Used
			processedDevices[p.Device] = true
		}
	}

	if total == 0 {
		return "未知", fmt.Errorf("无有效磁盘分区")
	}
	percent := (float64(used) / float64(total)) * 100
	return fmt.Sprintf("%.2f", percent), nil
}

// 操作系统名称处理器，返回操作系统的名称
func OSNameHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := host.Info() // 获取操作系统信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.Platform, nil // 返回操作系统名称
}

// 操作系统类型处理器，返回操作系统类型
func OSTypeHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := host.Info() // 获取操作系统信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.PlatformFamily, nil // 返回操作系统类型
}

// 操作系统版本处理器，返回操作系统版本
func OSVersionHandler(ctx context.Context, params map[string]string) (any, error) {
	info, err := host.Info() // 获取操作系统信息
	if err != nil {
		return "未知", err // 错误时返回“未知”
	}
	return info.PlatformVersion, nil // 返回操作系统版本
}
