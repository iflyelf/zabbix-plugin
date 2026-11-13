// 定义插件的配置结构和验证方法
package plugin

import (
    "golang.zabbix.com/sdk/conf"   // 导入配置解析模块
    "golang.zabbix.com/sdk/errs"   // 导入错误处理模块
    "golang.zabbix.com/sdk/plugin" // 导入插件模块
    "golang.zabbix.com/plugin/ports/plugin/params" // 导入 params 包
)

// 定义 session 结构体，用于会话信息
type session struct {
    Username string `conf:"optional"` // 用户名，可选
    Password string `conf:"optional"` // 密码，可选
}

// 定义 pluginConfig 结构体，用于存储插件配置
type pluginConfig struct {
    Timeout              int               `conf:"optional,range=1:30"` // 超时时间配置
    Sessions             map[string]session `conf:"optional"`           // 会话集合
    Default              session           `conf:"optional"`            // 默认会话
    ExcludedProcesses    string            `conf:"optional"`            // 排除进程配置项
}

// Configure：解析并初始化插件的配置
func (p *Plugin) Configure(global *plugin.GlobalOptions, options any) {
    pConfig := &pluginConfig{} // 初始化配置结构体

    // 使用 SDK 解析传入的配置选项
    err := conf.Unmarshal(options, pConfig)
    if err != nil {
        p.Errf("配置解析失败: %s", err.Error()) // 输出错误日志
        return
    }
    
    // 如果超时时间未设置，则使用全局超时时间
    if pConfig.Timeout == 0 {
        pConfig.Timeout = global.Timeout
    }
    
    // 解析并设置排除进程的列表
    params.SetExcludedProcesses(pConfig.ExcludedProcesses) // 将排除进程的列表传入参数模块

    // 保存解析后的配置
    p.config = pConfig

}

// Validate：验证配置是否正确
func (*Plugin) Validate(options any) error {
    var opts pluginConfig // 初始化临时配置结构体

    // 使用 SDK 解析配置选项
    err := conf.Unmarshal(options, &opts)
    if err != nil {
        return errs.Wrap(err, "配置解析失败") // 返回解析错误
    }

    return nil // 配置验证通过
}

// contains 函数检查字符串是否存在于给定的数组中
func contains(s []string, e string) bool {
    for _, v := range s {
        if v == e {
            return true
        }
    }
    return false
}
