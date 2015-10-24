package main

import "fmt"
import "net"
import "io"
import "flag"

var listen = flag.String("listen", "", "port (and optionally address) to listen on")
var speak = flag.String("speak", "", "address and port to connect to")

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
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("accept", err)
			return
		}
		handleConnection(conn)
	}
}

// Any time we get an inbound connection, connect to the "speak" host and port,
// and spawn two goroutines: one to copy data in each direction.
// When either connection generates an error (terminating the Copy call), close
// both connections.
func handleConnection(hearing net.Conn) {
	speaking, err := net.Dial("tcp", *speak)
	if err != nil {
		fmt.Println(err)
		return
	}
	go func() {
		_, err := io.Copy(hearing, speaking)
		if err != nil {
			fmt.Println(err)
		}
		hearing.Close()
		speaking.Close()
	}()
	go func() {
		_, err := io.Copy(speaking, hearing)
		if err != nil {
			fmt.Println(err)
		}
		hearing.Close()
		speaking.Close()
	}()
}
