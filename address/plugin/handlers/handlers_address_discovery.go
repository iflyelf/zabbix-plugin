// handlers_discovery.go
package handlers

import (
	"context"                                 // 用于插件上下文传递与控制
	"encoding/json"                           // 用于编码输出为 JSON 字符串
	"net"                                     // 提供系统网络接口查询能力
	"strings"                                 // 用于处理字符串匹配操作
	"golang.zabbix.com/sdk/errs"              // Zabbix 插件 SDK 的错误封装工具
)

// JSONOutput 定义 JSON 输出结构体，用于 Zabbix 自动发现格式
type JSONOutput struct {
	InterfaceName string   `json:"{#InterfaceName}"` // 网卡名称字段，Zabbix LLD 要求格式
	IPAddresses   []string `json:"{#IPAddresses}"`   // 网卡对应的 IPv4 地址列表
}

// isVirtualInterface 判断接口名称是否属于常见的虚拟网卡
func isVirtualInterface(name string) bool {
	// 常见虚拟网卡的前缀列表，可按需扩展
	virtualPrefixes := []string{
		"docker",   // Docker 虚拟网卡，如 docker0
		"br-",      // 桥接网卡，如 br-xxxxx
		"veth",     // 虚拟 Ethernet，如 vethxxxx
		"virbr",    // libvirt 虚拟网桥，如 virbr0
		"vmnet",    // VMware 网络接口，如 vmnet1
		"vboxnet",  // VirtualBox 网卡，如 vboxnet0
		"zt",       // ZeroTier 网络
		"tun",      // VPN 隧道接口，如 tun0
		"tap",      // 虚拟 TAP 接口
		"lo",       // loopback（虽然已经过滤，此处冗余保险）
		"cni",      // 容器网络接口（Kubernetes）
		"flannel",  // Kubernetes flannel 网络插件
		"ifb",      // 中间网卡（中断过滤）
	}
	// 遍历虚拟前缀，如果网卡名称以其开头则判定为虚拟接口
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// AddressDiscoveryHandler 是用于 Zabbix 自动发现的处理函数
func AddressDiscoveryHandler(ctx context.Context, _ map[string]string) (any, error) {
	// 获取本地所有网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, errs.Wrap(err, "获取网卡接口信息失败")
	}

	// 定义用于存放最终输出结果的切片
	var result []JSONOutput

	// 遍历所有网络接口
	for _, iface := range interfaces {
		// 条件 1：跳过 loopback（回环）接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// 条件 2：跳过未启用（down）的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		// 条件 3：跳过不支持多播的接口（大多物理接口都支持）
		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		// 条件 4：跳过被认为是虚拟网卡的接口
		if isVirtualInterface(iface.Name) {
			continue
		}

		// 获取该网卡的所有地址
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, errs.Wrapf(err, "获取网卡地址失败: %s", iface.Name)
		}

		// 定义 IP 地址列表（只收集 IPv4 地址）
		var ipAddresses []string
		for _, addr := range addrs {
			// 类型断言：是否为 IP 网络地址
			if ipnet, ok := addr.(*net.IPNet); ok {
				// 判断是否为 IPv4 地址
				if ipnet.IP.To4() != nil {
					ipAddresses = append(ipAddresses, ipnet.IP.String())
				}
			}
		}

		// 如果该网卡至少有一个 IPv4 地址，则添加到结果列表中
		if len(ipAddresses) > 0 {
			result = append(result, JSONOutput{
				InterfaceName: iface.Name,
				IPAddresses:   ipAddresses,
			})
		}
	}

	// 将结果结构转换为 JSON 字符串
	jsonResult, err := json.Marshal(result)
	if err != nil {
		return nil, errs.Wrap(err, "转换结果为 JSON 失败")
	}

	// 最终返回 JSON 字符串（类型为字符串）
	return string(jsonResult), nil
}
