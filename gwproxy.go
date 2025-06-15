package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"time"
)

var proto string
var bindAddr string
var targetAddr string

var dialer net.Dialer

func main() {
	var timeoutStr string
	var err error
	var conn net.Conn
	var listener net.Listener

	flag.StringVar(&proto, "proto", "tcp", "Protocol to use")
	flag.StringVar(&bindAddr, "from", "", "Bind address")
	flag.StringVar(&targetAddr, "to", "", "Proxy target")
	flag.StringVar(&timeoutStr, "timeout", "5s", "Timeout duration for upstream dial")

	flag.Parse()

	if len(bindAddr) == 0 || len(targetAddr) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	dialer.Timeout, err = time.ParseDuration(timeoutStr)

	if err != nil {
		log.Fatal("failed to parse timeout value:", err)
	}

	log.Printf("[Proto: %s] Now listening to %s", proto, bindAddr)
	listener, err = net.Listen(proto, bindAddr)

	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err = listener.Accept()
		if err != nil {
			log.Println("failed accepting incomming conn:", err)
			return
		}

		ip := conn.RemoteAddr().String()

		if tcpConn, ok := conn.(*net.TCPConn); ok {
			// This is a TCP connection. Establish NODELAY
			if err := tcpConn.SetNoDelay(true); err != nil {
				log.Println(ip, "client SetNoDelay(true) failed:", err)
			}
		}

		// Let different conn to handle it
		go handleConn(conn, ip)
	}
}

func handleConn(conn net.Conn, c_ip string) {
	defer conn.Close()

	upstream, err := net.Dial(proto, targetAddr)

	if err != nil {
		log.Println(c_ip, "failed dialing to upstream:", err)
		return
	}

	defer upstream.Close()

	if tcpConn, ok := upstream.(*net.TCPConn); ok {
		// This is a TCP connection. Establish NODELAY
		if err := tcpConn.SetNoDelay(true); err != nil {
			log.Println(c_ip, "upstream SetNoDelay(true) failed:", err)
		}
	}

	go feedToClient(upstream, conn) // conn -> upstream
	io.Copy(conn, upstream)         // conn <- upstream
}

func feedToClient(u, c net.Conn) {
	defer u.Close()

	io.Copy(u, c)
}
