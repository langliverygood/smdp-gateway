package datasource

import (
	"fmt"
	"net"
	"syscall"
)

type multicastConfig struct {
	IPMreq *syscall.IPMreq
	Fd     int
}

// joinMulticastGroup 加入组播组并监听消息
func joinMulticastGroup(ip string, conn *net.UDPConn) (*multicastConfig, error) {
	// 1. 解析组播地址
	// mcastUDPAddr, err := net.ResolveUDPAddr("udp", ip)
	// if err != nil {
	// 	return fmt.Errorf("解析组播地址失败: %w", err)
	// }

	// 调用前已经完成绑定, 由参数传递
	// 2. 绑定UDP端口（必须绑定，且端口与组播地址一致）
	// 注意：绑定地址为 0.0.0.0 表示接收所有网卡的组播流量
	// conn, err := net.ListenUDP("udp", &net.UDPAddr{
	// 	IP:   net.ParseIP(interfaceAddr),
	// 	Port: mcastUDPAddr.Port,
	// })
	// if err != nil {
	// 	return fmt.Errorf("绑定UDP端口失败: %w", err)
	// }
	// defer conn.Close()
	// fmt.Printf("已绑定UDP端口 %d，准备加入组播组 %s\n", mcastUDPAddr.Port, multicastAddr)

	// 3. 设置套接字选项，加入组播组
	// 核心：通过 syscall.SetsockoptIPMreq 配置组播组和本地网卡
	f, err := conn.File()
	if err != nil {
		return nil, fmt.Errorf("获取套接字文件描述符失败: %w", err)
	}
	defer f.Close()

	fd := int(f.Fd())
	// 构造 IPMreq 结构体（指定组播地址和本地网卡）
	ipMreq := syscall.IPMreq{
		Multiaddr: [4]byte{}, // 组播地址的4字节表示
		Interface: [4]byte{}, // 本地网卡地址（0.0.0.0 表示默认网卡）
	}

	// 解析组播地址到 Multiaddr
	copy(ipMreq.Multiaddr[:], net.ParseIP(ip).To4())
	// 解析本地网卡地址到 Interface（多网卡时替换为具体IP，如 192.168.1.100）
	// TODO
	copy(ipMreq.Interface[:], net.ParseIP("0.0.0.0").To4())

	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return nil, fmt.Errorf("加入组播组失败: %w", err)
	}

	// 加入组播组（setsockopt IP_ADD_MEMBERSHIP）
	err = syscall.SetsockoptIPMreq(fd, syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, &ipMreq)
	if err != nil {
		return nil, fmt.Errorf("加入组播组失败: %w", err)
	}
	// defer func() {
	// 	// 退出时离开组播组
	// 	syscall.SetsockoptIPMreq(fd, syscall.IPPROTO_IP, syscall.IP_DROP_MEMBERSHIP, &ipMreq)
	// 	fmt.Println("已离开组播组")
	// }()

	fmt.Println("成功加入组播组，" + ip)

	// // 4. 循环接收组播消息
	// buf := make([]byte, 1024)
	// for {
	// 	n, remoteAddr, err := conn.ReadFromUDP(buf)
	// 	if err != nil {
	// 		return fmt.Errorf("接收消息失败: %w", err)
	// 	}
	// 	fmt.Printf("收到来自 %s 的消息: %s\n", remoteAddr, string(buf[:n]))
	// }
	return &multicastConfig{
		IPMreq: &ipMreq,
		Fd:     fd,
	}, nil
}
