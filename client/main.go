package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"os"
)

func main() {
	localPort := flag.String("local", "5432", "local port to listen on")
	targetAddr := flag.String("target", "", "target address e.g. [2001:...]:5432")
	relayAddr := flag.String("relay", "13.207.62.103:28473", "relay server address")
	apiKey := flag.String("key", "", "API key for relay authentication")
	vpnMode := flag.Bool("vpn", false, "Start system-level VPN tunnel")

	flag.Parse()

	if *vpnMode {
		if os.Geteuid() != 0 {
			log.Fatal("sudo required for VPN boss")
		}
		startVPN(*relayAddr, *apiKey)
		return
	}
	

	if *targetAddr == "" {
		log.Fatal("You must specify a -target address!")
	}

	localListener, err := net.Listen("tcp", "127.0.0.1:"+*localPort)
	if err != nil {
		log.Fatal("Failed to listen: ", err)
	}
	defer localListener.Close()

	log.Printf("Listening on 127.0.0.1:%s -> Tunneling to %s\n", *localPort, *targetAddr)

	for {
		conn, err := localListener.Accept()
		if err != nil {
			continue
		}
		go handleLocalConn(conn, *relayAddr, *targetAddr, *apiKey)
	}
}

func handleLocalConn(localConn net.Conn, relayAddr, targetAddr, apiKey string) {
	defer localConn.Close()

	relayConn, err := tls.Dial("tcp", relayAddr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Println("Failed to connect to relay:", err)
		return
	}
	defer relayConn.Close()

	var req string
	if apiKey != "" {
		req = fmt.Sprintf("CONNECT %s HTTP/1.1\r\nX-API-Key: %s\r\n\r\n", targetAddr, apiKey)
	} else {
		req = fmt.Sprintf("CONNECT %s HTTP/1.1\r\n\r\n", targetAddr)
	}

	_, err = relayConn.Write([]byte(req))
	if err != nil {
		log.Println("Failed to send CONNECT request:", err)
		return
	}

	buff := make([]byte, 1024)
	n, err := relayConn.Read(buff)
	if err != nil || !strings.Contains(string(buff[:n]), "200") {
		log.Printf("Tunnel establishment failed: %s\n", string(buff[:n]))
		return
	}

	log.Printf("Tunnel established: 127.0.0.1 -> %s\n", targetAddr)

	go io.Copy(relayConn, localConn)
	io.Copy(localConn, relayConn)
}
