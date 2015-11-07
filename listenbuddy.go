package main

import "fmt"
import "net"
import "sync"
import "io"
import "flag"
import "os"
import "os/signal"
import "syscall"

var (
	listen  = flag.String("listen", "", "port (and optionally address) to listen on")
	speak   = flag.String("speak", "", "address and port to connect to")
	verbose = flag.Bool("v", false, "print errors that occur during copying of data")

	connections = make(map[net.Conn]struct{}, 100)
	cMu         sync.Mutex
)

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

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Println(err)
		return
	}
	go handleSignals()
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("accept", err)
			return
		}
		go handleConnection(conn)
	}
}

func handleSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	for _ = range ch {
		fmt.Println("closing")
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
		fmt.Printf("dial to %#v: %s\n", *speak, err)
		hearing.Close()
		return
	}
	addConnection(hearing)
	addConnection(speaking)
	ch := make(chan error, 1)
	go connCopy(hearing, speaking, ch)
	go connCopy(speaking, hearing, ch)
	<-ch
	hearing.Close()
	speaking.Close()
	removeConnection(hearing)
	removeConnection(speaking)
}

func connCopy(dst io.Writer, src io.Reader, dstCh chan<- error) {
	_, err := io.Copy(dst, src)
	if err != nil && *verbose {
		fmt.Println("copy:", err)
	}
	dstCh <- err
}
