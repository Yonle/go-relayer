package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
)

var proto string
var bindAddr string
var targetAddr string

func main() {
	flag.StringVar(&proto, "proto", "tcp", "Protocol to use")
	flag.StringVar(&bindAddr, "from", "", "Bind address")
	flag.StringVar(&targetAddr, "to", "", "Proxy target")

	flag.Parse()

	if len(bindAddr) == 0 || len(targetAddr) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	log.Printf("[Proto: %s] Now listening to %s", proto, bindAddr)
	l, err := net.Listen(proto, bindAddr)

	if err != nil {
		panic(err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("failed accepting incomming conn:", err)
			return
		}

		ip := conn.RemoteAddr().String()

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

	go feedToClient(upstream, conn) // conn -> upstream
	io.Copy(conn, upstream)         // conn <- upstream
}

func feedToClient(u, c net.Conn) {
	defer u.Close()

	io.Copy(u, c)
}
