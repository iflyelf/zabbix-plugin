// handlers_tcp_status.go：实现 TCP 连接状态的查询和处理逻辑
package handlers

import (
    "context" // 上下文管理，用于控制任务生命周期
    "encoding/json" // 导入 json 序列化模块，用于将整数转换为 JSON 格式
    "fmt"     // 格式化输出
    "os/exec" // 执行系统命令
    "strings" // 字符串处理，用于解析命令输出
)

// 定义 TCP 状态结构体，用于存储各种连接状态的计数
type TcpStatus struct {
    Listen    int // LISTEN 状态的连接数（正在监听的端口）
    Estab     int // ESTABLISHED 状态的连接数（已建立的连接）
    SynSent   int // SYN-SENT 状态的连接数（发送同步请求）
    SynRecv   int // SYN-RECV 状态的连接数（接收同步请求）
    FinWait1  int // FIN-WAIT-1 状态的连接数（第一次关闭等待）
    FinWait2  int // FIN-WAIT-2 状态的连接数（第二次关闭等待）
    TimeWait  int // TIME-WAIT 状态的连接数（等待关闭）
    CloseWait int // CLOSE-WAIT 状态的连接数（等待对方关闭）
    LastAck   int // LAST-ACK 状态的连接数（最后确认）
    Closing   int // CLOSING 状态的连接数（正在关闭）
    Unconn    int // 未知状态的连接数（无法识别的状态）
}

// 获取当前系统的 TCP 连接状态并进行统计
func getTcpStatus() TcpStatus {
    tcpStatus := TcpStatus{} // 初始化 TCP 状态结构体

    // 使用 ss 命令获取 TCP 连接信息（ss 工具更适合于新系统）
    cmd := exec.Command("ss", "-tan")
    output, err := cmd.Output() // 执行命令并获取输出
    if err != nil {
        fmt.Println("获取 TCP 状态失败:", err) // 打印错误信息
        return tcpStatus // 返回空的状态结构体
    }

    // 将命令输出按行分割为字符串数组
    lines := strings.Split(string(output), "\n")

    // 遍历每一行，统计各个状态的连接数
    for _, line := range lines[1:] {
        switch {
        case strings.Contains(line, "LISTEN"):
            tcpStatus.Listen++ // 统计 LISTEN 状态
        case strings.Contains(line, "ESTAB"):
            tcpStatus.Estab++ // 统计 ESTABLISHED 状态
        case strings.Contains(line, "SYN-SENT"):
            tcpStatus.SynSent++ // 统计 SYN-SENT 状态
        case strings.Contains(line, "SYN-RECV"):
            tcpStatus.SynRecv++ // 统计 SYN-RECV 状态
        case strings.Contains(line, "FIN-WAIT-1"):
            tcpStatus.FinWait1++ // 统计 FIN-WAIT-1 状态
        case strings.Contains(line, "FIN-WAIT-2"):
            tcpStatus.FinWait2++ // 统计 FIN-WAIT-2 状态
        case strings.Contains(line, "TIME-WAIT"):
            tcpStatus.TimeWait++ // 统计 TIME-WAIT 状态
        case strings.Contains(line, "CLOSE-WAIT"):
            tcpStatus.CloseWait++ // 统计 CLOSE-WAIT 状态
        case strings.Contains(line, "LAST-ACK"):
            tcpStatus.LastAck++ // 统计 LAST-ACK 状态
        case strings.Contains(line, "CLOSING"):
            tcpStatus.Closing++ // 统计 CLOSING 状态
        default:
            tcpStatus.Unconn++ // 统计无法识别的状态
        }
    }
    return tcpStatus // 返回统计结果
}

// TcpConnectStatusHandler 处理指定的 TCP 连接状态，返回所请求状态的计数
// 参数：ctx 为上下文对象，params 为查询时的参数（如状态名称）
func TcpConnectStatusHandler(ctx context.Context, params map[string]string) (any, error) {
    // 获取当前系统的 TCP 状态统计
    // 从参数中提取 "status_name" 以确定要查询的状态
    statusKey, ok := params["status_name"]
    if !ok {                                    // 检查是否提供了状态名称
        return nil, fmt.Errorf("缺少状态名称")    // 如果没有，返回错误
    }

    tcpStatus := getTcpStatus()                 // 调用 getTcpStatus 获取所有 TCP 状态的当前计数

    // 定义输出的结果为int类型
    var result int

    switch strings.ToLower(statusKey) {  // 根据状态名称选择对应的计数
    case "listen":
        result = tcpStatus.Listen // 返回 LISTEN 状态的连接数
    case "estab":
        result = tcpStatus.Estab // 返回 ESTABLISHED 状态的连接数
    case "syn-sent":
        result = tcpStatus.SynSent // 返回 SYN-SENT 状态的连接数
    case "syn-recv":
        result = tcpStatus.SynRecv // 返回 SYN-RECV 状态的连接数
    case "fin-wait-1":
        result = tcpStatus.FinWait1 // 返回 FIN-WAIT-1 状态的连接数
    case "fin-wait-2":
        result = tcpStatus.FinWait2 // 返回 FIN-WAIT-2 状态的连接数
    case "time-wait":
        result = tcpStatus.TimeWait // 返回 TIME-WAIT 状态的连接数
    case "close-wait":
        result = tcpStatus.CloseWait // 返回 CLOSE-WAIT 状态的连接数
    case "last-ack":
        result = tcpStatus.LastAck // 返回 LAST-ACK 状态的连接数
    case "closing":
        result = tcpStatus.Closing // 返回 CLOSING 状态的连接数
    case "unconn":
        result = tcpStatus.Unconn // 返回无法识别的状态计数
    default:
        return nil, fmt.Errorf("无效的状态名称: %s", statusKey) // 如果状态名称不正确，返回错误
    }

    // 将整数结果转换为 JSON 格式字符串，以确保传输过程中保持整数格式
    jsonResult, err := json.Marshal(result)     // 使用 json.Marshal 将整数结果编码为 JSON 格式
    if err != nil {                             // 检查编码过程是否出错
        return nil, fmt.Errorf("JSON 序列化失败: %s", err.Error()) // 如果出错，返回错误信息
    }

    // 返回 JSON 字符串，确保传输时保持数值格式
    return string(jsonResult), nil
}
