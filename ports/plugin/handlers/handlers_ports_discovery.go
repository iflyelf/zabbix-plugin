// handlers_ports_discovery.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/user"       // 用户信息获取
	"path/filepath" // 用于处理文件路径
	"runtime"       // 获取CPU核心数
	"strconv"       // 字符串转换
	"strings"       // 字符串处理
	"sync"          // 并发相关
	"time"          // 时间相关操作

	"golang.zabbix.com/plugin/ports/plugin/params" // 自定义参数包
)

// JSONOutput 定义Zabbix自动发现所需的JSON数据结构
type JSONOutput struct {
	ProcessName string `json:"{#PROCESS_NAME}"` // 进程名称
	Port        int    `json:"{#PORT}"`         // 监听端口号
	Pid         int    `json:"{#PID}"`          // 进程ID
	WorkDir     string `json:"{#WORK_DIR}"`     // 进程工作目录
	ExePath     string `json:"{#EXE_PATH}"`     // 可执行文件路径
	Username    string `json:"{#USERNAME}"`     // 运行用户
}

// GetTcpPortInfo 处理TCP端口发现请求
func GetTcpPortInfo(ctx context.Context, params map[string]string) (any, error) {
	return getPortInfoJSON("tcp")
}

// GetUdpPortInfo 处理UDP端口发现请求
func GetUdpPortInfo(ctx context.Context, params map[string]string) (any, error) {
	return getPortInfoJSON("udp")
}

// GetHttpPortInfo 处理HTTP端口发现请求（基于TCP端口检查）
func GetHttpPortInfo(ctx context.Context, params map[string]string) (any, error) {
	return getPortInfoJSON("http")
}

// getPortInfoJSON 通用端口信息获取方法
func getPortInfoJSON(protocol string) (any, error) {
	var ports []JSONOutput

	// 根据协议类型选择获取方式
	switch protocol {
	case "tcp":
		ports = getListeningPorts("tcp")
	case "udp":
		ports = getListeningPorts("udp")
	case "http":
		ports = getListeningHttpPorts() // HTTP需要额外检查
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	// 将结果转换为JSON字符串
	jsonData, err := json.Marshal(ports)
	if err != nil {
		return nil, fmt.Errorf("JSON marshal failed: %v", err)
	}
	return string(jsonData), nil
}

// getListeningHttpPorts 获取所有HTTP端口（从TCP端口筛选）
func getListeningHttpPorts() []JSONOutput {
	tcpPorts := getListeningPorts("tcp")
	var httpPorts []JSONOutput

	// 遍历TCP端口检查HTTP响应
	for _, port := range tcpPorts {
		if checkHTTPResponse(port.Port) {
			httpPorts = append(httpPorts, port)
		}
	}
	return httpPorts
}

// getListeningPorts 极致优化：只遍历有监听socket的inode，反查pid，极大减少遍历量
// 实现思路：
// 1. 先解析 /proc/net/tcp*，建立所有监听socket的 inode->port 映射表。
// 2. 统计所有监听socket的inode集合，便于快速判断。
// 3. 并发遍历所有 /proc/[pid]/fd 目录，只查找 fd 指向的 inode 是否在监听集合里。
// 4. 命中后反查出 pid、端口、进程名、用户等信息，端口去重，极大减少无效遍历和I/O。
func getListeningPorts(protocol string) []JSONOutput {
	var results []JSONOutput
	var seen sync.Map // 用于端口去重，确保每个端口只记录一次

	// 1. 解析 /proc/net/tcp*，建立 inode->port 映射
	inodePortMap := buildInodePortMap(protocol)
	if len(inodePortMap) == 0 {
		return results // 没有监听socket，直接返回空
	}

	// 2. 反查所有监听socket的inode属于哪个pid
	// 建立 port->[]inode 映射，便于后续端口去重（可选，当前主要用 inodePortMap）
	portInodes := make(map[int][]string)
	for inode, port := range inodePortMap {
		portInodes[port] = append(portInodes[port], inode)
	}

	// 3. 统计所有监听socket的inode集合，便于快速判断
	inodeSet := make(map[string]struct{})
	for inode := range inodePortMap {
		inodeSet[inode] = struct{}{}
	}

	// 4. 自动调整并发数：默认为CPU核心数一半，向上取整，最小1，最大128。可通过环境变量PORT_DISCOVERY_MAX_CONCURRENCY覆盖。
	maxConcurrency := (runtime.NumCPU() + 1) / 2 // 向上取整
	if maxConcurrency < 1 {
		maxConcurrency = 1 // 保底
	}
	if maxConcurrency > 128 {
		maxConcurrency = 128 // 封顶，防止极端大主机
	}
	// 支持通过环境变量PORT_DISCOVERY_MAX_CONCURRENCY覆盖
	if v := os.Getenv("PORT_DISCOVERY_MAX_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConcurrency = n
		}
	}
	// maxConcurrency为本次端口发现的最大并发数，默认为CPU核心数一半，既能利用多核，又能避免端口发现任务占满所有CPU，影响业务进程。可通过环境变量PORT_DISCOVERY_MAX_CONCURRENCY覆盖。

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency) // 动态并发控制
	resultCh := make(chan JSONOutput, 1024)    // 用于收集结果

	entries, err := os.ReadDir("/proc")
	if err != nil {
		fmt.Printf("读取/proc失败: %v\n", err)
		return results
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // 跳过非目录
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // 跳过非数字目录
		}
		sem <- struct{}{} // 控制并发数
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			defer func() { <-sem }()
			fdPath := fmt.Sprintf("/proc/%d/fd", pid)
			fds, err := os.ReadDir(fdPath)
			if err != nil {
				return // 无权限或目录不存在则跳过
			}
			for _, fdEntry := range fds {
				fdLink := fmt.Sprintf("%s/%s", fdPath, fdEntry.Name())
				link, err := os.Readlink(fdLink)
				if err != nil || !strings.HasPrefix(link, "socket:[") {
					continue // 跳过非socket链接
				}
				inode := link[8 : len(link)-1] // 提取socket inode号
				if _, ok := inodeSet[inode]; !ok {
					continue // 只处理监听socket
				}
				port, ok := inodePortMap[inode]
				if !ok || port == 0 {
					continue // 未找到端口则跳过
				}
				key := strconv.Itoa(port)
				if _, exists := seen.LoadOrStore(key, true); exists {
					continue // 端口去重，确保每个端口只记录一次
				}
				exePath := getProcessExe(pid)
				if isExcludedProcess(exePath) {
					continue // 排除指定进程
				}
				// 命中后收集完整信息
				resultCh <- JSONOutput{
					ProcessName: filepath.Base(exePath),
					Port:        port,
					Pid:         pid,
					WorkDir:     getProcessCwd(pid),
					ExePath:     exePath,
					Username:    getProcessUser(pid),
				}
			}
		}(pid)
	}

	// 等待所有并发任务完成，关闭结果通道
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 主线程收集所有结果
	for r := range resultCh {
		results = append(results, r)
	}
	return results
}

// buildInodePortMap 一次性解析协议文件，返回 inode->port 映射表
func buildInodePortMap(protocol string) map[string]int {
	var protoFiles []string
	switch protocol {
	case "tcp":
		protoFiles = []string{"/proc/net/tcp", "/proc/net/tcp6"}
	case "udp":
		protoFiles = []string{"/proc/net/udp", "/proc/net/udp6"}
	default:
		return map[string]int{}
	}
	m := make(map[string]int)
	for _, file := range protoFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue // 文件读取失败则跳过
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines[1:] { // 跳过标题行
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue // 无效数据行
			}
			// 对于TCP协议检查LISTEN状态（0A为十六进制表示）
			if protocol == "tcp" && len(fields) >= 4 && fields[3] != "0A" {
				continue // 跳过非监听状态连接
			}
			inode := fields[9]
			addrParts := strings.Split(fields[1], ":")
			if len(addrParts) < 2 {
				continue // 无效地址格式
			}
			portHex := addrParts[len(addrParts)-1]
			port, err := strconv.ParseInt(portHex, 16, 32)
			if err != nil {
				continue // 转换失败则跳过
			}
			m[inode] = int(port)
		}
	}
	return m
}

// findPortByInode 通过inode查找端口号（已废弃，仅保留兼容性，实际不再调用）
func findPortByInode(inode string, protocol string) int {
	return 0 // 已不再使用
}

// checkHTTPResponse 验证端口是否响应HTTP请求
func checkHTTPResponse(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second} // 2秒超时
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		return false // 请求失败则非HTTP
	}
	defer resp.Body.Close()
	return len(resp.Header) > 0 // 有响应头则视为HTTP服务
}

// getProcessExe 获取进程的可执行文件路径
func getProcessExe(pid int) string {
	path, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)) // 读取符号链接
	return path
}

// getProcessCwd 获取进程的工作目录
func getProcessCwd(pid int) string {
	path, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return "/" // 默认返回根目录
	}
	return path
}

// getProcessUser 获取进程的运行用户
func getProcessUser(pid int) string {
	content, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}

	// 在status文件中查找Uid行
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				// 通过用户ID获取用户名
				u, err := user.LookupId(fields[1])
				if err == nil {
					return u.Username
				}
			}
		}
	}
	return ""
}

// isExcludedProcess 判断是否是需要排除的进程
func isExcludedProcess(exePath string) bool {
	// 合并预设和自定义排除列表
	excluded := append(params.GetExcludedProcesses(),
		"systemd", "init", "kthreadd", "ksoftirqd", "rcu_sched", "rcu_bh", "bo-agent",
		"migration", "watchdog", "cpuhp", "netns", "khungtaskd", "oom_reaper",
		"kswapd", "fsnotify_mark", "ecryptfs-kthrea", "kauditd", "khubd", "ssh",
		"zabbix", "prometheus", "rpcbind", "smartdns", "cupsd", "dhclient", "master",
		"rpc.statd", "titanagent", "node_exporter", "monitor_manage", "dnsmasq",
	)

	// 提取可执行文件名并检查排除列表
	exeName := filepath.Base(exePath)
	for _, proc := range excluded {
		if strings.Contains(exeName, proc) {
			return true
		}
	}
	return false
}
