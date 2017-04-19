package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var listen = flag.String("listen", "", "port (and optionally address) to listen on")
var speak = flag.String("speak", "", "address and port to connect to")

var connections = make(map[*net.TCPConn]struct{}, 100)
var cMu sync.Mutex

func main() {
	flag.Parse()
	if *listen == "" || *speak == "" {
		fmt.Println(`
Forwards all connections from a given port to a different address and port.
Example:
	listenbuddy -listen :8000 -speak localhost:80
`)
		flag.Usage()
		return
	}
	log.SetPrefix("listenbuddy ")

	speakAddr, err := net.ResolveTCPAddr("tcp", *speak)
	if err != nil {
		log.Println(err)
		return
	}

	listenAddr, err := net.ResolveTCPAddr("tcp", *listen)
	if err != nil {
		log.Println(err)
		return
	}
	ln, err := net.ListenTCP("tcp", listenAddr)
	if err != nil {
		log.Println(err)
		return
	}
	go handleSignals()
	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			log.Println("accept", err)
			return
		}
		go handleConn(speakAddr, conn)
	}
}

func handleSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	for _ = range ch {
		log.Println("closing")
		closeAllConnections()
	}
}

func closeAllConnections() {
	cMu.Lock()
	defer cMu.Unlock()
	for c, _ := range connections {
		c.CloseWrite()
	}
}

func addConnection(c *net.TCPConn) {
	cMu.Lock()
	connections[c] = struct{}{}
	cMu.Unlock()
}

func removeConnection(c *net.TCPConn) {
	cMu.Lock()
	delete(connections, c)
	cMu.Unlock()
}

func copyConn(dst, src *net.TCPConn) {
	addConnection(src)
	_, err := io.Copy(dst, src)
	if err != nil {
		// We commonly get use of closed network connection when a server shuts down
		// active
		if !strings.Contains(err.Error(), "use of closed network connection") {
			log.Println(err)
		}
	}
	src.Close()
	dst.CloseWrite()
	removeConnection(src)
}

// Any time we get an inbound connection, connect to the "speak" host and port,
// and spawn two goroutines: one to copy data in each direction.
// When either connection generates an error (terminating the Copy call), close
// both connections.
func handleConn(speakAddr *net.TCPAddr, hearing *net.TCPConn) {
	speaking, err := net.DialTCP("tcp", nil, speakAddr)
	if err != nil {
		log.Println(err)
		hearing.Close()
		return
	}
	go copyConn(speaking, hearing)
	copyConn(hearing, speaking)
}
