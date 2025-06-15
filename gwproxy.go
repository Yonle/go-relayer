package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"syscall"
	"time"
)

var proto string
var ListenAddr string
var targetAddr string
var timeout time.Duration

var dialer net.Dialer
var listener net.ListenConfig

func main() {
	var timeoutStr string
	var keepAlive bool
	var keepAlive_IdleStr string
	var keepAlive_IntervalStr string

	var bindAddr string

	flag.StringVar(&proto, "proto", "tcp", "Protocol to use")
	flag.StringVar(&ListenAddr, "from", "", "Listen to address")
	flag.StringVar(&targetAddr, "to", "", "Upstream target address")
	flag.StringVar(&timeoutStr, "timeout", "5s", "Timeout duration for upstream dial")

	flag.BoolVar(&keepAlive, "keepalive", false, "Enable KeepAlive (TCP)")
	flag.StringVar(&keepAlive_IdleStr, "keepalive-idle", "15s", "Keep Alive idle duration")
	flag.StringVar(&keepAlive_IntervalStr, "keepalive-interval", "15s", "Keep Alive interval duration")

	flag.StringVar(&bindAddr, "bind", "", "Dial to upstream with specified local IP address (Bind)")

	flag.Parse()

	if len(ListenAddr) == 0 || len(targetAddr) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	keepAliveConf := net.KeepAliveConfig{}
	keepAliveConf.Enable = keepAlive
	keepAliveConf.Idle = parseDur(keepAlive_IdleStr, "keepalive-idle")
	keepAliveConf.Interval = parseDur(keepAlive_IntervalStr, "keepalive-interval")

	dialer.KeepAliveConfig = keepAliveConf

	if len(bindAddr) != 0 {
		dialer.LocalAddr = &net.IPAddr{
			IP: net.ParseIP(bindAddr),
		}
	}

	timeout = parseDur(timeoutStr, "timeout")

	listener.KeepAliveConfig = keepAliveConf

	prepareRLimit()
	startListening()
}

func prepareRLimit() {
	// Ref:
	// https://github.com/alviroiskandar/gwproxy/blob/4090c92d911c89acf5ac95fa8a9e96ec444837c1/gwproxy.c#L133
	var rlim syscall.Rlimit

	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim); err != nil {
		log.Printf("Warning: Unable to get RLimit: %v", err)
		return
	}

	rlim.Cur = rlim.Max

	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlim); err != nil {
		log.Printf("Warning: Unable to raise RLIMIT: %v", err)
	}

	log.Printf("RLIMIT_NOFILE set to %d", rlim.Cur)
}

func startListening() {
	var listen net.Listener
	var err error

	log.Printf("[Proto: %s] Now listening to %s", proto, ListenAddr)
	listen, err = listener.Listen(context.Background(), proto, ListenAddr)

	if err != nil {
		log.Fatal(err)
	}

	var conn net.Conn

	for {
		conn, err = listen.Accept()
		if err != nil {
			log.Println("failed accepting incomming conn:", err)
			continue
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
	defer conn.Close() // close client after copy

	upstream, err := dialer.DialContext(makeDeadlineCtx(), proto, targetAddr)

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

	go feedToClient(conn, upstream) // conn <- upstream
	io.Copy(upstream, conn)         // upstream <- conn
}

func feedToClient(c, u net.Conn) {
	defer u.Close() // close upstream after copy

	io.Copy(c, u)
}

func parseDur(t, k string) (d time.Duration) {
	var err error
	d, err = time.ParseDuration(t)

	if err != nil {
		log.Printf("Failed to parse duration value of %s: %v", k, err)
	}

	return
}

func makeDeadlineCtx() (ctx context.Context) {
	ctx, _ = context.WithDeadlineCause(
		context.Background(),
		time.Now().Add(timeout),
		fmt.Errorf("Connection to upstream is timed out."),
	)
	return
}
