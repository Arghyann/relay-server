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

var (
	serverTun   *water.Interface
	serverTunMu sync.Mutex
)

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
		return nil, fmt.Errorf("failed to assign IP: %v", err)
	}

	cmd = exec.Command("ip", "link", "set", ifce.Name(), "up")
	if err := cmd.Run(); err != nil {
		ifce.Close()
		return nil, fmt.Errorf("failed to bring interface UP: %v", err)
	}

	exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()

	ensureIptablesRule("nat", "POSTROUTING", "-s", "198.18.0.0/24", "-j", "MASQUERADE")
	ensureIptablesRule("", "FORWARD", "-s", "198.18.0.0/24", "-j", "ACCEPT")
	ensureIptablesRule("", "FORWARD", "-d", "198.18.0.0/24", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	ensureIptablesRule("mangle", "FORWARD", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")

	serverTun = ifce
	return serverTun, nil
}

func handleVPNConn(conn net.Conn) {
	tun, err := initServerTun()
	if err != nil {
		log.Println("VPN setup error: ", err)
		conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
		return
	}

	conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	log.Println("VPN session established")

	go func() {
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
	}()

	packet := make([]byte, 1500)
	for {
		n, err := tun.Read(packet)
		if err != nil {
			return
		}
		binary.Write(conn, binary.BigEndian, uint16(n))
		_, err = conn.Write(packet[:n])
		if err != nil {
			log.Println("VPN client disconnected")
			return
		}
	}
}
