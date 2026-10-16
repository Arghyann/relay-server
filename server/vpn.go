package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"sync"

	"github.com/songgao/water"
)

type ipPool struct {
	mu   sync.Mutex
	used map[int]bool
}

func (p *ipPool) Acquire() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 2; i <= 254; i++ {
		if !p.used[i] {
			p.used[i] = true
			return i, nil
		}
	}
	return 0, fmt.Errorf("all IP ranges taken, try again later")
}

func (p *ipPool) Release(i int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.used, i)
}

var (
	serverTun     *water.Interface
	serverTunMu   sync.Mutex
	pool          = &ipPool{used: make(map[int]bool)}
	activeClients sync.Map
)

func startCentralTunReader(tun *water.Interface) {
	go func() {
		packet := make([]byte, 1500)
		for {
			n, err := tun.Read(packet)
			if err != nil {
				return
			}
			var destIP string
			version := packet[0] >> 4 // check for v4 and v6
			if version == 4 && n >= 20 {
				destIP = net.IP(packet[16:20]).String()
			} else if version == 6 && n >= 40 {
				destIP = net.IP(packet[24:40]).String()
			} else {
				continue
			}
			if clientVal, ok := activeClients.Load(destIP); ok {
				clientConn := clientVal.(net.Conn)
				binary.Write(clientConn, binary.BigEndian, uint16(n))
				clientConn.Write(packet[:n])
			}
		}
	}()
}

func initServerTun() (*water.Interface, error) {
	serverTunMu.Lock()
	defer serverTunMu.Unlock()

	if serverTun != nil {
		return serverTun, nil
	}
	config := water.Config{DeviceType: water.TUN}
	config.Name = "relay%d"
	ifce, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create server TUN: %v", err)
	}

	log.Printf("Server TUN interface created: %s\n", ifce.Name())

	cmd := exec.Command("ip", "addr", "add", "198.18.0.1/24", "dev", ifce.Name())
	if err := cmd.Run(); err != nil {
		ifce.Close()
		return nil, fmt.Errorf("failed to assign IPv4: %v", err)
	}

	cmd = exec.Command("ip", "-6", "addr", "add", "fd00:198:18::1/64", "dev", ifce.Name())
	if err := cmd.Run(); err != nil {
		ifce.Close()
		return nil, fmt.Errorf("failed to assign IPv6: %v", err)
	}

	cmd = exec.Command("ip", "link", "set", ifce.Name(), "up")
	if err := cmd.Run(); err != nil {
		ifce.Close()
		return nil, fmt.Errorf("failed to bring interface UP: %v", err)
	}

	exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
	exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run()

	ensureIptablesRule("nat", "POSTROUTING", "-s", "198.18.0.0/24", "-j", "MASQUERADE")
	ensureIptablesRule("", "FORWARD", "-s", "198.18.0.0/24", "-j", "ACCEPT")
	ensureIptablesRule("", "FORWARD", "-d", "198.18.0.0/24", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	ensureIptablesRule("mangle", "FORWARD", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")

	ensureIp6tablesRule("nat", "POSTROUTING", "-s", "fd00:198:18::/64", "-j", "MASQUERADE")
	ensureIp6tablesRule("", "FORWARD", "-s", "fd00:198:18::/64", "-j", "ACCEPT")
	ensureIp6tablesRule("", "FORWARD", "-d", "fd00:198:18::/64", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	ensureIp6tablesRule("mangle", "FORWARD", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")

	serverTun = ifce
	startCentralTunReader(serverTun)
	return serverTun, nil
}

func handleVPNConn(conn net.Conn) {
	tun, err := initServerTun()
	if err != nil {
		log.Println("VPN setup error: ", err)
		conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
		return
	}

	id, err := pool.Acquire()
	if err != nil {
		log.Println("Pool error: ", err)
		conn.Write([]byte("HTTP/1.1 503 Service Unavailable\r\n\r\n"))
		return
	}
	defer pool.Release(id)

	clientIPv4 := fmt.Sprintf("198.18.0.%d", id)
	clientIPv6 := fmt.Sprintf("fd00:198:18::%d", id)

	activeClients.Store(clientIPv4, conn)
	activeClients.Store(clientIPv6, conn)
	defer func() {
		activeClients.Delete(clientIPv4)
		activeClients.Delete(clientIPv6)
		log.Printf("VPN client [%s, %s] disconnected\n", clientIPv4, clientIPv6)
	}()

	resp := fmt.Sprintf("HTTP/1.1 200 Connection Established\r\nX-Client-IPv4: %s/24\r\nX-Client-IPv6: %s/64\r\n\r\n", clientIPv4, clientIPv6)
	conn.Write([]byte(resp))
	log.Printf("VPN session established for [%s, %s]\n", clientIPv4, clientIPv6)

	// Stream packets from Client -> Server TUN
	for {
		var length uint16
		err := binary.Read(conn, binary.BigEndian, &length)
		if err != nil {
			return
		}
		packet := make([]byte, length)
		_, err = io.ReadFull(conn, packet)
		if err != nil {
			return
		}
		tun.Write(packet)
	}
}
