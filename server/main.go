package main

import (
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
)

func main() {
	addKeyName := flag.String("add-key", "", "Generate a new API key for the given name and exit")
	listKeys := flag.Bool("list-keys", false, "List all active API keys and exit")
	port := flag.String("port", "28473", "Port to listen on")
	flag.Parse()

	db, err := createDb()
	if err != nil {
		log.Fatal("Couldn't init DB: ", err)
	}
	defer db.Close()

	if *addKeyName != "" {
		key, err := addKey(db, *addKeyName)
		if err != nil {
			log.Fatal("Failed to create key: ", err)
		}
		fmt.Printf("Created API Key for [%s]: %s\n", *addKeyName, key)
		return
	}

	if *listKeys {
		if err := listAllKeys(db); err != nil {
			log.Fatal("Failed to list keys: ", err)
		}
		return
	}

	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Fatal("Failed to load keys: ", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&count)
	if count == 0 {
		key, _ := addKey(db, "admin")
		log.Println("==================================================")
		log.Println("Generated initial API Key for [admin]:", key)
		log.Println("==================================================")
	}

	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err := tls.Listen("tcp", ":"+*port, config)
	if err != nil {
		log.Fatal("Couldn't start tcp: ", err)
	}
	defer listener.Close()

	log.Printf("Listening on Port %s\n", *port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Ran into issues while handling connection: ", err)
			continue
		}
		go handleConn(conn, db)
	}
}

func handleConn(conn net.Conn, db *sql.DB) {
	defer conn.Close()
	log.Println("New Connection from:", conn.RemoteAddr())

	buff := make([]byte, 1024)
	n, err := conn.Read(buff)
	if err != nil || n < 1 {
		log.Println("Failed to read from conn: ", err)
		return
	}

	var method, target, protocol string
	fmt.Sscanf(string(buff[:n]), "%s %s %s", &method, &target, &protocol)
	if method != "CONNECT" {
		log.Println("Unsupported method: ", method)
		return
	}

	reqStr := string(buff[:n])
	var clientKey string
	for _, line := range strings.Split(reqStr, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "x-api-key:") {
			clientKey = strings.TrimSpace(line[len("x-api-key:"):])
			break
		}
	}

	username, ok := validateKey(db, clientKey)
	if !ok {
		log.Printf("[%s] Unauthorized connection rejected\n", conn.RemoteAddr())
		conn.Write([]byte("HTTP/1.1 401 Unauthorized\r\n\r\n"))
		return
	}

	log.Printf("[%s] Authorized user: %s -> %s\n", conn.RemoteAddr(), username, target)

	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		log.Println("Failed to connect to target: ", target, err)
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	go io.Copy(targetConn, conn)
	io.Copy(conn, targetConn)
}
