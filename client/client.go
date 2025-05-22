/* server/server.go has explenations for nearly all lines of non-trivial code that won't be repeated here */

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	pb "rpc-chat/chat"

	"google.golang.org/grpc"
)

var (
	wait   *sync.WaitGroup
	client pb.BroadcastClient
	name   = flag.String("name", "Anonymous", "The username you wish to display")
	port   = flag.String("port", "50051", "The port to connect to")
)

func connect(user *pb.User) error {
	var streamerror error

	stream, err := client.CreateStream(context.Background(), &pb.Connect{
		User:   user,
		Active: true,
	})

	if err != nil {
		return fmt.Errorf("Connection failed - %v", err)
	}

	wait.Add(1)
	go func(str pb.Broadcast_CreateStreamClient) {
		defer wait.Done()

		for {
			msg, err := str.Recv()
			if err != nil {
				streamerror = fmt.Errorf("Error reading message - %v", err)
				break
			}

			fmt.Printf("%v : %s\n", msg.ClientId, msg.Value)
		}
	}(stream)

	return streamerror
}

func main() {
	flag.Parse()

	wait = &sync.WaitGroup{}

	timestamp := time.Now()
	done := make(chan int)

	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		fmt.Errorf("Error connecting to server - %v", err)
	}

	client = pb.NewBroadcastClient(conn)

	id := sha256.Sum256([]byte(timestamp.String() + *name))
	user := &pb.User{
		Name:     *name,
		ClientId: hex.EncodeToString(id[:]),
	}

	connect(user)

	wait.Add(1)
	go func() {
		defer wait.Done()

		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			msg := &pb.Message{
				ClientId:  user.ClientId,
				Value:     scanner.Text(),
				Timestamp: timestamp.String(),
			}

			_, err := client.BroadcastMessage(context.Background(), msg)
			if err != nil {
				fmt.Errorf("Error sending message to server - %v", err)
				break
			}
		}
	}()

	go func() {
		wait.Wait()
		close(done)
	}()

	<-done
}
