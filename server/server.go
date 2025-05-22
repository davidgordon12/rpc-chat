package main

import (
	"flag"
	"fmt"
	"net"
)

var port = flag.Int("port", 50051, "The server port")

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
}
