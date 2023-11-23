package main

import (
	"context"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"strconv"
	"time"
)

type MessageType int

const (
	REQUEST    MessageType = iota // for client to make a request to server
	RELEASE                       // for client to tell server that client is done with cs
	OK_RELEASE                    // for server to tell client that release is successful
	WAIT                          // for server to tell client to wait
	OK_ENTER                      // for server to signal to client to enter cs
)

type Message struct {
	MessageType MessageType
	ClientID    int
	LeaderID    int
	RequestID   int
}

var (
	message      Message
	requestCount int
)

func SendRequest(requestCount int) {
	// var err error

	// err = client.Call("Node.HandleRequest", &message, &reply)
	// if err != nil {
	// 	log.Fatal(err)
	// 	log.Println("Error connecting to Server. Trying to connect to Backup Server")
	// 	go contactBackup()
	// }
	// fmt.Println("Received:", reply)

	// Create leader based on requestCount
	// By default it will use the leader request
	// If the first request fails, it will attempt to send the request again to the leader
	// If the second request fails, it will contact the backup
	var leader string
	if requestCount < 3 {
		leader = "LEADER_ADDRESS"
	} else if requestCount == 3{
		leader = "BACKUP_ADDRESS"
	} else {
		fmt.Println("Not handled")
		return
	}
	requestCount ++
	nodeID, _ := strconv.Atoi(os.Getenv("NODE_ID"))
	leaderAddress := os.Getenv(leader) // Assume the format is "node2:8080"
	client, err := rpc.Dial("tcp", leaderAddress)
	if err != nil {
		log.Fatal("Failed to connect to leader:", err)
	}
	defer client.Close()
	fmt.Println("Client", nodeID, "connected to server")

	// Declare a timeout for the request
	var reply MessageType
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		err := client.Call("Node.HandleRequest", &message, &reply)
		done <- err
	}()

	// Wait for the call to finish or the timeout
	select {
	case <-ctx.Done():
		fmt.Println("Request timed out")
		SendRequest(client, )
	case err := <-done:
		if err != nil {
			fmt.Printf("RPC call failed: %v\n", err)
			fmt.Println("Error connecting to Server. Trying to connect to Backup Server")
		} else {
			fmt.Println("RPC call succeeded, reply:", reply)
			if reply == WAIT {
				fmt.Println("Waiting")
			} else if reply == OK_ENTER {
				fmt.Println("Holding lock")
			}
		}
	}
	// select {
	// case reply := <-c:
	// 	if reply == WAIT {
	// 		fmt.Println("Waiting")
	// 	} else if reply == OK_ENTER {
	// 		fmt.Println("Holding lock")
	// 	}
	// case <- time.After(500 * time.Nanosecond):
	// 	RetryRequest(client)
	// }
}

func SendRelease(client *rpc.Client) {
	var err error
	var reply MessageType
}

func RequestLock(client *rpc.Client, nodeID int) {
	message.ClientID = nodeID
	message.LeaderID = 2

	message.MessageType = REQUEST
	message.RequestID = 1
	requestCount = 1
	fmt.Println("Client", nodeID, "sent Request", message.RequestID, "for lock")
	SendRequest(client, requestCount)

	// Some critical section function
	time.Sleep(time.Second)

	message.MessageType = RELEASE
	fmt.Println("Client", nodeID, "sent Release lock")
	SendRequest(client)

	// Queued requests
	time.Sleep(time.Second)

	message.Body = "REQUEST"
	message.RequestID += 1
	fmt.Println("Client", nodeID, "sent Request", message.RequestID, "for lock")
	SendRequest(client)

	time.Sleep(time.Second)

	message.Body = "RELEASE"
	fmt.Println("Client", nodeID, "sent Release lock")
	SendRequest(client)

	// New requests
	time.Sleep(4 * time.Second)

	message.Body = "REQUEST"
	message.RequestID += 1
	fmt.Println("Client", nodeID, "sent Request", message.RequestID, "for lock")
	SendRequest(client)

	time.Sleep(time.Second)

	message.Body = "RELEASE"
	fmt.Println("Client", nodeID, "sent Release lock")
	SendRequest(client)
}

// func contactServer() {
// 	nodeID, _ := strconv.Atoi(os.Getenv("NODE_ID"))
// 	leaderAddress := os.Getenv("LEADER_ADDRESS") // Assume the format is "node2:8080"
// 	client, err := rpc.Dial("tcp", leaderAddress)
// 	if err != nil {
// 		log.Fatal("Failed to connect to leader:", err)
// 	}
// 	defer client.Close()
// 	fmt.Println("Client", nodeID, "connected to server")
// 	go RequestLock(client, nodeID)
// 	time.Sleep(30 * time.Second)
// }

// func contactBackup() {
// 	nodeID, _ := strconv.Atoi(os.Getenv("NODE_ID"))
// 	backupAddress := os.Getenv("BACKUP_ADDRESS") // Assume the format is "node2:8080"
// 	client, err := rpc.Dial("tcp", backupAddress)
// 	if err != nil {
// 		log.Fatal("Failed to connect to backup  server:", err)
// 	}
// 	defer client.Close()
// 	fmt.Println("Client", nodeID, "connected to backup server")
// 	go RequestLock(client, nodeID)
// 	time.Sleep(30 * time.Second)
// }

func main() {
	time.Sleep(7 * time.Second)
	go contactServer()
	time.Sleep(50 * time.Second)
}
