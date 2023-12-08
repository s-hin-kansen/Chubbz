package main

import (
	"context"
	"fmt"
	"io"
	"log"

	// "math/rand"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"time"
)

type ClientMessageType int

const (
	REQUEST    ClientMessageType = iota // for client to make a request to server
	RELEASE                             // for client to tell server that client is done with cs
	OK_RELEASE                          // for server to tell client that release is successful
	WAIT                                // for server to tell client to wait
	OK_ENTER                            // for server to signal to client to enter cs
)

// Request information for server-client communications
type Request struct {
	ClientID  int
	RequestID int
}

type Message struct {
	ClientMessageType ClientMessageType
	LeaderID          int
	Request           Request
}

var (
	nodeID           int
	message          Message
	requestCount     int
	requestID        int
	requestCompleted int
	leader           string
)

// Split into request for lock & release lock
func SendRequest(
	requestCount int,
	requestType ClientMessageType,
	requestID int,
) {
	// Create leader based on requestCount
	// By default it will use the leader request
	// If it is the second request, it will attempt to send the request again to the leader
	// If it is the third request, it will contact the backup
	if leader == "LEADER_ADDRESS" {
		if requestCount < 3 {
			message.LeaderID = 2
		} else if requestCount == 3 {
			leader = "BACKUP_ADDRESS"
			message.LeaderID = 1
		} else {
			fmt.Println("Not handled")
			return
		}
	} else {
		message.LeaderID = 1
	}
	// Set message fields
	message.Request.RequestID = requestID
	message.Request.ClientID = nodeID
	message.ClientMessageType = requestType

	// Dial server
	leaderAddress := os.Getenv(leader) // Assume the format is "node2:8080"
	client, err := rpc.Dial("tcp", leaderAddress)
	if err != nil {
		log.Fatal("Failed to connect to leader:", err)
	}
	defer client.Close()
	fmt.Println("Client", nodeID, "connected to server")

	// Declare a timeout for the request
	var reply ClientMessageType
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		// fmt.Printf("Sending: %+v\n", message.ClientMessageType)
		err := client.Call("Node.HandleRequest", &message, &reply)
		done <- err
	}()

	// Wait for the call to finish or the timeout
	if requestType == REQUEST {
		fmt.Println("Client", nodeID, "sent REQUEST", message.Request.RequestID, "to Leader", message.LeaderID, "for lock")
	} else if requestType == RELEASE {
		fmt.Println("Client", nodeID, "sent RELEASE", message.Request.RequestID, "to Leader", message.LeaderID)
	} else {
		fmt.Println("Not handled")
		return
	}

	// Increase requestCount by 1
	requestCount++

	// Handle timeout
	select {
	case <-ctx.Done():
		fmt.Println("Request timed out")
		go SendRequest(requestCount, requestType, requestID)
	case err := <-done:
		if err != nil {
			fmt.Printf("RPC call failed: %v\n", err)
			fmt.Println("Error connecting to Server. Trying to connect to Backup Server")
			SendRequest(1, requestType, requestID)
		} else {
			fmt.Println("RPC call succeeded, reply:", reply)
			if reply == WAIT {
				fmt.Println("Client", nodeID, "received WAIT from Leader", message.LeaderID)
				receivedEnter := false
				// // Start go function with timeout to wait for OK_ENTER from server
				// // If no reply received within 5 seconds, request timeouts and proceeds to retry request
				// ctx2 := listenWithTimeout(client, &receivedEnter)
				// select {
				// case <-ctx2.Done():
				// 	if !receivedEnter {
				// 		fmt.Println("Request timed out")
				// 		SendRequest(requestCount, requestType, requestID)
				// 	}
				// }
				listenForServerMessages(&receivedEnter)
			} else if reply == OK_ENTER {
				fmt.Println("Entering Critical section")

				time.Sleep(2 * time.Second)
				SendRequest(1, RELEASE, requestID)
			} else if reply == OK_RELEASE {
				fmt.Println("Client", nodeID, "received OK_RELEASE from Leader", message.LeaderID)
				requestCompleted = requestID
			} else if reply == 0 {
				fmt.Println("Error connecting to Server. Trying again")
				SendRequest(requestCount, requestType, requestID)
			}
		}
	}
}

// // Request to enter again, health check for the server
// func listenWithTimeout(client *rpc.Client, receivedEnter *bool) context.Context {
// 	// Declare a timeout for the request
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()
// 	listenForServerMessages(receivedEnter)
// 	return ctx
// }

// Open listener for "OK ENTER"
func listenForServerMessages(receivedEnter *bool) {
	log.Println("Listening on 8080")
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("Server message failed", err)
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		log.Println(err)
	}
	handleServerMessage(conn, receivedEnter)
}

// Handling of "OK ENTER"
func handleServerMessage(conn net.Conn, receivedEnter *bool) {
	defer conn.Close()
	tmp := make([]byte, 256) // using small tmo buffer for demonstrating
	for {
		_, err := conn.Read(tmp)
		if err != nil {
			if err != io.EOF {
				fmt.Println("read error:", err)
			}
			break
		}
		fmt.Println("Client", nodeID, "received OK_ENTER from Leader")
		// Some critical section function
		time.Sleep(2 * time.Second)
		*receivedEnter = true
		SendRequest(1, RELEASE, requestID)
		break
	}
}

func listenForLeaderElection() {
	ln, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatal("Server message failed", err)
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		log.Println(err)
	}

	defer conn.Close()
	tmp := make([]byte, 256) // using small tmo buffer for demonstrating
	for {
		output, err := conn.Read(tmp)
		if err != nil {
			if err != io.EOF {
				fmt.Println("read error:", err)
			}
			break
		}
		fmt.Println("New leader elected:", output)
		leader = "BACKUP_ADDRESS"
		break
	}
}

func main() {
	nodeID, _ = strconv.Atoi(os.Getenv("NODE_ID"))
	// Wait for server goroutines to initialise
	// time.Sleep(7 * time.Second)

	leader = "LEADER_ADDRESS"
	// Start request for lock
	requestCount = 1
	requestID = 1
	requestCompleted = 0

	go listenForLeaderElection()

	// Request for lock after

	time.Sleep(5 * time.Second)
	go SendRequest(requestCount, REQUEST, requestID)
	time.Sleep(1000 * time.Second)

	// for {
	// 	go SendRequest(requestCount, REQUEST, requestID)
	// 	for requestCompleted != requestID {
	// 		time.Sleep(time.Second)
	// 	}
	// 	requestID++

	// 	// Define the range for the timeout (e.g., between 1 and 5 seconds)
	// 	// minTimeout := 1
	// 	// maxTimeout := 5

	// 	// Generate a random duration within the range
	// 	// randomDuration := time.Duration(rand.Intn(maxTimeout-minTimeout+1)+minTimeout) * time.Second

	// 	time.Sleep(10 * time.Second)
	// }
}
