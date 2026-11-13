// handlers_pid_status.go
package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.zabbix.com/sdk/errs"
)

// 标准映射表：英文key <-> 标准中文描述（仅保留能通过 /proc 采集的指标，rss/vsz 改为 vmrss/vmsize）
var pidstatDict = map[string]string{
	"%cpu":         "进程使用cpu的使用率",
	"minflt/s":     "从内存中加载数据时每秒出现的次要错误的数目",
	"majflt/s":     "从内存中加载数据时每秒出现的主要错误的数目",
	"vmrss":        "进程使用的物理内存",
	"vmsize":       "进程使用的虚拟内存",
	"%mem":         "进程使用物理内存的使用率",
	"kb_rd/s":      "进程每秒从磁盘读取的数据量",
	"kb_wr/s":      "进程每秒向磁盘写入的数据量",
	"threads":      "进程线程总数",
	"voluntary":    "每秒主动任务上下文切换的次数",
	"nonvoluntary": "每秒被动任务上下文切换的次数",
}

// 自动生成反向映射：标准中文描述 -> 英文key
var zhKeyMap = map[string]string{}

func init() {
	for k, v := range pidstatDict {
		zhKeyMap[v] = k
	}
	// 常用别名
	zhKeyMap["线程数"] = "threads"
	zhKeyMap["物理内存"] = "vmrss"
	zhKeyMap["虚拟内存"] = "vmsize"
	zhKeyMap["rss"] = "vmrss"
	zhKeyMap["vsz"] = "vmsize"
	zhKeyMap["cpu使用率"] = "%cpu"
	zhKeyMap["物理内存使用率"] = "%mem"
}

// 全局缓存结构
var (
	pidCache   = make(map[int]*procCache)
	cacheMutex sync.RWMutex
)

type procCache struct {
	lastTime       time.Time
	lastUtime      float64
	lastStime      float64
	lastMinflt     float64
	lastMajflt     float64
	lastReadBytes  float64
	lastWriteBytes float64
}

// 清理参数字符串，统一小写、去空格
func cleanMetricName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// GetPidStatus 主入口函数
func GetPidStatus(ctx context.Context, metricParams map[string]string) (interface{}, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[pid_status] panic recovered: %v\n", r)
		}
	}()
	metricName, metricExists := metricParams["metric"]
	pidStr, pidExists := metricParams["pid"]

	if !metricExists || !pidExists {
		return nil, errs.New("缺少 metric 或 pid 参数")
	}

	cleaned := cleanMetricName(metricName)
	// 1. 先查标准中文描述
	for k, v := range pidstatDict {
		if cleanMetricName(v) == cleaned {
			cleaned = k
			break
		}
	}
	// 2. 再查别名
	if mapped, ok := zhKeyMap[cleaned]; ok {
		cleaned = mapped
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("无效的 PID: %s", pidStr)
	}

	data, err := getPidData(pid)
	if err != nil {
		return nil, err
	}

	fmt.Printf("[GetPidStatus] cleaned: '%s', data: %+v\n", cleaned, data)
	for k, v := range data {
		if cleanMetricName(k) == cleaned {
			return v, nil
		}
	}
	fmt.Printf("[pid_status] 未匹配到指标: '%s' 字节: %v, 可用: %+v\n", metricName, []byte(metricName), data)
	return nil, fmt.Errorf("无效的指标名称: '%s'，可用指标: %v", metricName, data)
}

// 核心数据获取逻辑
func getPidData(pid int) (map[string]float64, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	now := time.Now()
	cache, exists := pidCache[pid]
	newCache := &procCache{lastTime: now}

	// 获取基础状态信息
	statusData, err := getStatus(pid)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[getPidData] statusData: %+v\n", statusData)

	// 获取CPU和缺页信息
	utime, stime, minflt, majflt, err := getCPUAndFaults(pid)
	if err != nil {
		return nil, err
	}

	// 获取IO数据
	readBytes, writeBytes, err := getIOStats(pid)
	if err != nil {
		return nil, err
	}

	// 初始化缓存数据
	newCache.lastUtime = utime
	newCache.lastStime = stime
	newCache.lastMinflt = minflt
	newCache.lastMajflt = majflt
	newCache.lastReadBytes = readBytes
	newCache.lastWriteBytes = writeBytes

	// 计算增量值
	var result = make(map[string]float64)
	if exists {
		timeDiff := now.Sub(cache.lastTime).Seconds()
		if timeDiff > 0 {
			result["%cpu"] = ((utime - cache.lastUtime) + (stime - cache.lastStime)) / timeDiff
			result["minflt/s"] = (minflt - cache.lastMinflt) / timeDiff
			result["majflt/s"] = (majflt - cache.lastMajflt) / timeDiff
			result["kb_rd/s"] = (readBytes - cache.lastReadBytes) / 1024 / timeDiff
			result["kb_wr/s"] = (writeBytes - cache.lastWriteBytes) / 1024 / timeDiff
		}
	} else {
		// 首次采集返回0值
		result["%cpu"] = 0
		result["minflt/s"] = 0
		result["majflt/s"] = 0
		result["kb_rd/s"] = 0
		result["kb_wr/s"] = 0
	}

	// 获取内存总量
	memTotal, _ := getMemTotal()
	if memTotal == 0 {
		memTotal = 1 // 防止除零
	}

	// 静态指标
	result["vmsize"] = parseStatusValue(statusData["vmsize"])
	result["vmrss"] = parseStatusValue(statusData["vmrss"])
	result["threads"] = parseStatusValue(statusData["threads"])
	result["voluntary"] = parseStatusValue(statusData["voluntary_ctxt_switches"])
	result["nonvoluntary"] = parseStatusValue(statusData["nonvoluntary_ctxt_switches"])
	result["%mem"] = result["vmrss"] / memTotal * 100

	fmt.Printf("[getPidData] result: %+v\n", result)

	// 更新缓存
	pidCache[pid] = newCache
	return result, nil
}

// 从/proc/[pid]/status获取状态信息
func getStatus(pid int) (map[string]string, error) {
	statusFile := fmt.Sprintf("/proc/%d/status", pid)
	file, err := os.Open(statusFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		data[key] = value
	}
	fmt.Printf("[getStatus] data: %+v\n", data)
	return data, scanner.Err()
}

// 从/proc/[pid]/stat获取CPU和缺页信息
func getCPUAndFaults(pid int) (float64, float64, float64, float64, error) {
	content, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, 0, 0, err
	}

	fields := strings.Fields(string(content))
	if len(fields) < 24 {
		return 0, 0, 0, 0, fmt.Errorf("invalid stat format")
	}

	utime, _ := strconv.ParseFloat(fields[13], 64)
	stime, _ := strconv.ParseFloat(fields[14], 64)
	minflt, _ := strconv.ParseFloat(fields[9], 64)
	majflt, _ := strconv.ParseFloat(fields[11], 64)

	return utime / 100, stime / 100, minflt, majflt, nil
}

// 从/proc/[pid]/io获取IO数据
func getIOStats(pid int) (float64, float64, error) {
	content, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/io", pid))
	if err != nil {
		return 0, 0, err
	}

	var readBytes, writeBytes float64
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "read_bytes:") {
			fmt.Sscanf(line, "read_bytes: %f", &readBytes)
		}
		if strings.HasPrefix(line, "write_bytes:") {
			fmt.Sscanf(line, "write_bytes: %f", &writeBytes)
		}
	}
	return readBytes, writeBytes, nil
}

// 获取系统内存总量
func getMemTotal() (float64, error) {
	content, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "MemTotal:") {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 {
				return strconv.ParseFloat(fields[1], 64)
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found")
}

// 辅助函数：转换状态值，支持带单位（如 '25592 kB'）
func parseStatusValue(value string) float64 {
	if value == "" {
		return 0
	}
	// 只取第一个数字部分
	fields := strings.Fields(value)
	f, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		fmt.Printf("[parseStatusValue] 转换失败: '%s'\n", value)
		return 0
	}
	return f
}

// 辅助函数：安全转换字符串
func safeParseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
