package main

import (
	"fmt"
	"log"
	"net"
	"crypto/tls"
	"io"
)

func main(){

	cert,err:=tls.LoadX509KeyPair("cert.pem","key.pem")
	if err!=nil{
		log.Fatal("Failed to load keys")
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err := tls.Listen("tcp",":28473",config)
	if err!=nil{
		log.Fatal("Couldn't start tcp: ",err)
	}
	defer listener.Close()
	log.Println("Listening on Port 28473")
	for {
		conn,err:=listener.Accept()
		if err!=nil{
			log.Println("Ran into issues while handling connection",err)
			continue
		}
		go handleConn(conn)
	}
}





func handleConn(conn net.Conn){
	defer conn.Close()
	log.Println("New Connection from:",conn.RemoteAddr())
	buff:=make([]byte,1024)
	n,err:=conn.Read(buff)
	if err!=nil || n<1 {
		log.Println("Failed to read from conn")
		return 
	}

	var method, target, protocol string

	fmt.Sscanf(string(buff[:n]),"%s %s %s",&method,&target,&protocol)
  if method!="CONNECT"{
		log.Println("Unsupported method: ",method)
		return
	}
	log.Println("Connecting to Target: ", target)
	
	targetConn,err:=net.Dial("tcp",target)

	if err!=nil{
		log.Println("Failed to connect to target: ", target)
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return 
	}
	defer targetConn.Close()
	conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	go io.Copy(targetConn,conn)
	io.Copy(conn,targetConn)

}	


