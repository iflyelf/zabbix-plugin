package main

import (
	"fmt"
	"net"
	"os"
)

func outboundIPv4() (string, error) {
	conn, err := net.Dial("udp4", "8.8.8.8:53")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || local.IP == nil {
		return "", fmt.Errorf("无法获取本地地址")
	}
	ip4 := local.IP.To4()
	if ip4 == nil {
		return "", fmt.Errorf("未获得 IPv4 地址")
	}
	return ip4.String(), nil
}

func main() {
	ip, err := outboundIPv4()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(ip)
}
