package main

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"os"
	"os/signal"
	"syscall"

	"github.com/songgao/water"
)

func createTunInterface(ipWithSubnet string) (*water.Interface, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = "relay%d"
	ifce, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to open TUN interface: %v", err)
	}
	log.Println("Created Ephemeral Network Interface: ", ifce.Name())
	cmd := exec.Command("ip", "addr", "add", ipWithSubnet, "dev", ifce.Name())
	err = cmd.Run()
	if err != nil {
		ifce.Close()
		return nil, fmt.Errorf("failed to assign IP address: %v", err)
	}
	cmd = exec.Command("ip", "link", "set", ifce.Name(), "up")
	err = cmd.Run()
	if err != nil {
		ifce.Close()
		return nil, fmt.Errorf("failed to bring interface up: %v", err)
	}
	return ifce, nil
}

func startVPN(relayAddr string, apiKey string) {
	tun, err := createTunInterface("198.18.0.2/24")
	if err != nil {
		log.Fatal(err)
	}
	defer tun.Close()
	gw,dev,err:=getGatewayFor(relayAddr)
	if err!=nil{
		log.Fatal("Failed to detect gateway: ",err)
	}

	relayConn, err := tls.Dial("tcp", relayAddr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Fatal("Failed to connect to relay: ", err)
	}
	defer relayConn.Close()

	req := fmt.Sprintf("CONNECT /vpn HTTP/1.1\r\nX-API-Key: %s\r\n\r\n", apiKey)
	relayConn.Write([]byte(req))

	buff := make([]byte, 1024)
	n, err := relayConn.Read(buff)
	if err != nil || !strings.Contains(string(buff[:n]), "200") {
		log.Fatalf("Server rejected VPN handshake: %s", string(buff[:n]))
	}
	log.Println("VPN Tunnel established! Streaming raw IP packets...")
	setupRoutes(relayAddr,tun.Name(),gw,dev)
	defer cleanupRoutes(relayAddr,tun.Name(),gw,dev)
	sigchan := make(chan os.Signal,1)
	signal.Notify(sigchan,os.Interrupt,syscall.SIGTERM)
	go func(){
		<-sigchan
		log.Println("Shutting down VPN")
		cleanupRoutes(relayAddr,tun.Name(),gw,dev)
		tun.Close()
		relayConn.Close()
		os.Exit(0)

	}()
	// Stream A: Packets from OS (tun) -> encrypt and send to Relay over TLS
	go func() {
		packet := make([]byte, 1500)
		for {
			n, err := tun.Read(packet)
			if err != nil {
				return
			}
			binary.Write(relayConn, binary.BigEndian, uint16(n))
			relayConn.Write(packet[:n])
		}
	}()

	// Stream B: Packets from Relay -> inject into OS (tun)
	for {
		var length uint16
		err = binary.Read(relayConn, binary.BigEndian, &length)
		if err != nil {
			log.Println("Connection to relay lost: ", err)
			return
		}
		packet := make([]byte, length)
		_, err = io.ReadFull(relayConn, packet)
		if err != nil {
			log.Println("Read packet error: ", err)
			return
		}
		tun.Write(packet)
	}
}
