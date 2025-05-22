package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	pb "rpc-chat/chat"

	"google.golang.org/grpc"
	glog "google.golang.org/grpc/grpclog"
)

var grpcLog glog.LoggerV2

var port = flag.Int("port", 50051, "The server port")

type Connection struct {
	stream pb.Broadcast_CreateStreamServer
	id     string
	active bool
	/* Needs to be a channel type since we will be using goroutines */
	error chan error
}

type Server struct {
	pb.UnimplementedBroadcastServer
	Connection []*Connection
}

/* Opens a stream between a client (the caller) and the server.
 * This method doesn't need to return a tuple with the result type
 * because we stream the result to the client */
func (s *Server) CreateStream(conn *pb.Connect, stream pb.Broadcast_CreateStreamServer) error {
	// Create a connection for the client that is calling this function
	connection := &Connection{
		stream: stream,
		id:     conn.User.ClientId,
		active: true,
		error:  make(chan error),
	}
	s.Connection = append(s.Connection, connection)

	return <-connection.error
}

/* Takes in a gRPC context and our defined message type.
 * Returns our defined Close type and an error */
func (s *Server) BroadcastMessage(ctx context.Context, msg *pb.Message) (*pb.Close, error) {
	/* This is a counter that allows us to count the amount of goroutines we have.
	 * Once a goroutine finishes, we decrement the counter. This way we can block
	 * until a goroutine has finished */
	wait := sync.WaitGroup{}

	/* We will use this to know when all of our goroutines are finished */
	done := make(chan int)

	/* Handle all of our connections */
	for _, conn := range s.Connection {
		wait.Add(1)

		go func(msg *pb.Message, conn *Connection) {
			defer wait.Done() // Decrement the "counter" once the goroutine is done

			if conn.active {
				grpcLog.Info("Sending message to: ", conn.stream)
				if err := conn.stream.Send(msg); err != nil {
					grpcLog.Error("Error with stream: %s", err.Error())
					conn.active = false
					conn.error <- err
				}
			}
		}(msg, conn)

	}

	/* Block until all the goroutines are finished */
	go func() {
		wait.Wait()
		/* Issue a close command to the done channel */
		close(done)
	}()

	/* Block the return of this function until the wait group ends */
	<-done
	return &pb.Close{}, nil
}

func main() {
	flag.Parse()
	grpcLog = glog.NewLoggerV2(os.Stdout, os.Stdout, os.Stderr)

	var connections []*Connection
	server := &Server{pb.UnimplementedBroadcastServer{}, connections}

	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("Error opening server - %v", err)
	}
	defer listener.Close()

	grpcLog.Info("Started server on port ", *port)
	pb.RegisterBroadcastServer(grpcServer, server)
	grpcServer.Serve(listener)
}
