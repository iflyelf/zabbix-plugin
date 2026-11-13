// handlers_conntrack_status.go：实现 conntrack 状态的查询和处理逻辑
package handlers

import (
    "context"      // 上下文管理，用于控制任务生命周期
    "encoding/json" // 导入 json 序列化模块，用于将整数转换为 JSON 格式
    "fmt"          // 格式化输出
    "io/ioutil"    // 读文件工具
    "strconv"      // 字符串与整数的转换
    "strings"      // 字符串处理，用于去除空白字符
)

// 读取文件并解析为整数
func readIntFromFile(filePath string) (int, error) {
    data, err := ioutil.ReadFile(filePath) // 读取文件内容
    if err != nil {
        return 0, err // 返回错误
    }
    str := strings.TrimSpace(string(data)) // 去除多余空白
    value, err := strconv.Atoi(str) // 转换字符串为整数
    return value, err
}

// ConntrackInfoHandler 处理 conntrack.info 指标，返回阈值比较结果
func ConntrackInfoHandler(ctx context.Context, params map[string]string) (any, error) {
    nfConntrackMax, err := readIntFromFile("/proc/sys/net/nf_conntrack_max")
    if err != nil {
        return nil, fmt.Errorf("读取 nf_conntrack_max 出错: %v", err)
    }
    nfConntrackNumber, err := readIntFromFile("/proc/sys/net/netfilter/nf_conntrack_count")
    if err != nil {
        return nil, fmt.Errorf("读取 nf_conntrack_count 出错: %v", err)
    }
    threshold := float64(nfConntrackMax) * 0.7 // 设置阈值为最大值的70%
    result := 1
    if float64(nfConntrackNumber) > threshold {
        result = 0
    }
    jsonResult, _ := json.Marshal(result)
    return string(jsonResult), nil
}

// ConntrackNumberHandler 处理 conntrack.number 指标，返回当前连接数
func ConntrackNumberHandler(ctx context.Context, params map[string]string) (any, error) {
    nfConntrackNumber, err := readIntFromFile("/proc/sys/net/netfilter/nf_conntrack_count")
    if err != nil {
        return nil, fmt.Errorf("读取 nf_conntrack_count 出错: %v", err)
    }
    jsonResult, _ := json.Marshal(nfConntrackNumber)
    return string(jsonResult), nil
}
