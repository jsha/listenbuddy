package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var listen = flag.String("listen", "", "port (and optionally address) to listen on")
var speak = flag.String("speak", "", "address and port to connect to")

var connections = make(map[net.Conn]struct{}, 100)
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

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Println(err)
		return
	}
	go handleSignals()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("accept", err)
			return
		}
		handleConnection(conn)
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
		c.Close()
	}
}

func addConnection(c net.Conn) {
	cMu.Lock()
	connections[c] = struct{}{}
	cMu.Unlock()
}

func removeConnection(c net.Conn) {
	cMu.Lock()
	delete(connections, c)
	cMu.Unlock()
}

// Any time we get an inbound connection, connect to the "speak" host and port,
// and spawn two goroutines: one to copy data in each direction.
// When either connection generates an error (terminating the Copy call), close
// both connections.
func handleConnection(hearing net.Conn) {
	speaking, err := net.Dial("tcp", *speak)
	if err != nil {
		log.Println(err)
		hearing.Close()
		return
	}
	addConnection(hearing)
	addConnection(speaking)
	go func() {
		_, err := io.Copy(hearing, speaking)
		if err != nil {
			fmt.Println(err)
		}
		hearing.Close()
		speaking.Close()
		removeConnection(speaking)
	}()
	go func() {
		_, err := io.Copy(speaking, hearing)
		if err != nil {
			fmt.Println(err)
		}
		hearing.Close()
		speaking.Close()
		removeConnection(speaking)
	}()
}
