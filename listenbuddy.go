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
		go handleConnection(conn)
	}
}

func makeChan(r io.Reader) chan []byte {
	c := make(chan []byte, 10)
	go readToChan(r, c)
	return c
}

func readToChan(r io.Reader, c chan []byte) {
	for {
		buf := make([]byte, 512)
		n, err := r.Read(buf)
		if err != nil {
			if err.Error() != "EOF" {
				fmt.Println("read", err)
				return
			}
		}
		c <- buf[0:n]
	}
}

func handleConnection(hearing net.Conn) {
	speaking, err := net.Dial("tcp", *speak)
	if err != nil {
		fmt.Println(err)
		return
	}
	hearingChan := makeChan(hearing)
	backtalkChan := makeChan(speaking)
	for {
		select {
		case heard := <-hearingChan:
			n, err := speaking.Write(heard)
			if err != nil {
				fmt.Println("hear", err)
				return
			}
			if n != len(heard) {
				fmt.Println("short write")
				return
			}
		case replied := <-backtalkChan:
			n, err := hearing.Write(replied)
			if err != nil {
				fmt.Println("backtalk", err)
				return
			}
			if n != len(replied) {
				fmt.Println("short write")
				return
			}
		}
	}
}
