package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var proto string
var ListenAddr string
var targetAddr string
var timeout time.Duration

var clientBufferSize int
var upstreamBufferSize int

var dialer net.Dialer
var listenconf net.ListenConfig
var listener net.Listener

var gctx context.Context
var gctx_cancel context.CancelFunc

type Session struct {
	clientCloseOnce   sync.Once
	upstreamCloseOnce sync.Once
	wg                sync.WaitGroup

	Client   net.Conn
	ClientIP string
}

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
	flag.IntVar(&clientBufferSize, "clientbuffersize", 4096, "Client buffer size in bytes")
	flag.IntVar(&upstreamBufferSize, "upstreambuffersize", 4096, "Upstream buffer size in bytes")

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
	listenconf.KeepAliveConfig = keepAliveConf

	makeGlobalCtx()
	prepareRLimit()
	startListening()
}

func makeGlobalCtx() {
	gctx, gctx_cancel = context.WithCancel(context.Background())
	go listenForSignal()
}

func listenForSignal() {
	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c // Synchronous: Wait for signal

	log.Println("SIGINT/SIGTERM received.")

	gctx_cancel()
	log.Println("Global context has been cancelled.")

	if listener != nil {
		log.Println("Waiting for all remaining connections to close...")
		listener.Close()
	}
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
	var err error

	log.Printf("[Proto: %s] Now listening to %s", proto, ListenAddr)
	listener, err = listenconf.Listen(gctx, proto, ListenAddr)

	if err != nil {
		log.Fatal(err)
	}

	var conn net.Conn

	for {
		conn, err = listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
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
		s := Session{
			Client:   conn,
			ClientIP: ip,
		}

		s.handle()
	}
}

func (s *Session) handle() {
	ctx, cancel := makeDeadlineCtx()

	defer s.clientCloseOnce.Do(func() { s.Client.Close() }) // close client after copy. Call this one time and only.
	defer cancel()

	upstream, err := dialer.DialContext(ctx, proto, targetAddr)

	if err != nil {
		if gctx_err := gctx.Err(); gctx_err != nil {
			return
		}
		log.Println(s.ClientIP, "failed dialing to upstream:", err)
		return
	}

	cancel()
	defer s.upstreamCloseOnce.Do(func() { upstream.Close() })

	if tcpConn, ok := upstream.(*net.TCPConn); ok {
		// This is a TCP connection. Establish NODELAY
		if err := tcpConn.SetNoDelay(true); err != nil {
			log.Println(s.ClientIP, "upstream SetNoDelay(true) failed:", err)
		}
	}

	s.wg.Add(2)

	go s.feedStream(s.Client, upstream) // conn -> upstream
	go s.feedStream(upstream, s.Client)

	s.wg.Wait() // wait till all of them closes.
}

func (s *Session) feedStream(dst, src net.Conn) {
	defer s.wg.Done()
	buf := make([]byte, upstreamBufferSize)

	io.CopyBuffer(dst, src, buf)
	if dst_tcpConn, ok := dst.(*net.TCPConn); ok {
		dst_tcpConn.CloseWrite() // we close write.
	}
}

func parseDur(t, k string) (d time.Duration) {
	var err error
	d, err = time.ParseDuration(t)

	if err != nil {
		log.Fatalf("Failed to parse duration value of %s: %v", k, err)
	}

	return
}

func makeDeadlineCtx() (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithDeadlineCause(
		gctx,
		time.Now().Add(timeout),
		fmt.Errorf("Connection to upstream is timed out."),
	)
	return
}
