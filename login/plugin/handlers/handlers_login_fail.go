// handlers_login_fail.go 定义登录失败信息查询的处理函数
package handlers

import (
    "context"                     // 提供上下文管理功能，便于控制操作的生命周期
    "fmt"                         // 用于格式化字符串输出
    "os/exec"                     // 用于执行外部命令
    "strconv"                     // 用于字符串与整数之间的转换
    "strings"                     // 用于字符串操作
    "time"                        // 提供时间操作，如获取当前时间
)

// LoginFailedAttemptsHandler 查询并返回登录失败尝试次数
func LoginFailedAttemptsHandler(ctx context.Context, params map[string]string) (any, error) {
    // 获取当前的时间（日期和小时:分钟）
    currentTime := time.Now()
    day := fmt.Sprintf("%02d", currentTime.Day()) // 获取当前日期并格式化为两位数
    hourMinute := currentTime.Format("15:04") // 格式化当前时间为"小时:分钟"

    // 尝试获取 lastb 登录失败次数
    lastbCount, err := getLastbFailedAttempts(day, hourMinute)
    if err != nil {
        // 如果 lastb 命令失败，尝试使用日志文件的方法
        logFileCount, logErr := getLogFileFailedAttempts(day, hourMinute)
        if logErr != nil {
            return 0, fmt.Errorf("获取登录失败次数时出错: %v, %v", err, logErr)
        }
        return logFileCount, nil
    }

    // 返回 lastb 的结果
    return lastbCount, nil
}

// getLastbFailedAttempts 使用 lastb 命令获取登录失败次数
func getLastbFailedAttempts(day, hourMinute string) (int, error) {
    cmd := exec.Command("bash", "-c", fmt.Sprintf("lastb | grep '%s %s' | wc -l", day, hourMinute))

    output, err := cmd.Output()
    if err != nil {
        return 0, fmt.Errorf("执行 lastb 命令失败: %v", err)
    }

    // 解析命令输出
    countStr := strings.TrimSpace(string(output))
    count, err := strconv.Atoi(countStr) // 将输出转换为整数
    if err != nil {
        return 0, fmt.Errorf("转换登录失败次数时出错: %v", err)
    }

    return count, nil // 返回登录失败次数
}

// getLogFileFailedAttempts 从日志文件中获取登录失败次数
func getLogFileFailedAttempts(day, hourMinute string) (int, error) {
    cmd := exec.Command("bash", "-c", fmt.Sprintf(
        "find /var/log/ -iname 'secure' -or -iname 'auth.log' -or -iname 'messages' | xargs grep '%s %s' | egrep 'Failed password' | wc -l", day, hourMinute))

    output, err := cmd.Output()
    if err != nil {
        return 0, fmt.Errorf("执行日志查找命令失败: %v", err)
    }

    // 解析命令输出
    countStr := strings.TrimSpace(string(output))
    count, err := strconv.Atoi(countStr) // 将输出转换为整数
    if err != nil {
        return 0, fmt.Errorf("转换登录失败次数时出错: %v", err)
    }

    return count, nil // 返回登录失败次数
}

