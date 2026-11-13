// params.go：定义插件参数和排除进程的设置
package params

import (
    "strings"                      // 字符串处理包
    "golang.zabbix.com/sdk/metric" // 引入 Zabbix SDK 的 metric 包
)

// 定义参数名称的常量
const (
    UsernameParameterName = "Username" // 用户名参数
    PasswordParameterName = "Password" // 密码参数
)

// 定义连接参数的列表，包括用户名和密码
var (
    Params = []*metric.Param{Username, Password} // 基本连接参数列表

    // 用户名参数，默认值为 "Zabbix"
    Username = metric.NewConnParam(
        UsernameParameterName, "模拟用户名参数。",
    ).WithSession().WithDefault("Zabbix")

    // 密码参数
    Password = metric.NewConnParam(PasswordParameterName, "模拟密码参数。")

    // ExcludedProcesses 用于存储配置的排除进程列表
    ExcludedProcesses = []string{"rpcbind"} // 默认排除 "rpcbind" 进程
)

// SetExcludedProcesses 从配置文件设置排除进程列表
func SetExcludedProcesses(excludedProcesses string) {
    if excludedProcesses == "" {
        ExcludedProcesses = []string{"rpcbind"} // 如果未设置则使用默认值
    } else {
        ExcludedProcesses = strings.Split(excludedProcesses, ",") // 用逗号分割
    }
}

// GetExcludedProcesses 返回排除进程列表
func GetExcludedProcesses() []string {
    return ExcludedProcesses
}

