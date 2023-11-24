package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
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

// Request information for server-client communications
type Request struct {
	ClientID  int
	RequestID int
}

type Message struct {
	MessageType MessageType
	LeaderID    int
	Request     Request
}

var (
	nodeID           int
	message          Message
	requestCount     int
	requestID        int
	requestCompleted int
)

func SendRequest(requestCount int, requestType MessageType, requestID int) {
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
	// If it is the second request, it will attempt to send the request again to the leader
	// If it is the third request, it will contact the backup
	var leader string
	if requestCount < 3 {
		leader = "LEADER_ADDRESS"
		message.LeaderID = 2
	} else if requestCount == 3 {
		leader = "BACKUP_ADDRESS"
		message.LeaderID = 1
	} else {
		fmt.Println("Not handled")
		return
	}

	// Set message fields
	message.Request.RequestID = requestID
	message.Request.ClientID = nodeID
	message.MessageType = requestType

	// Dial server
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
		fmt.Printf("Sending: %+v\n", message.MessageType)
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
		SendRequest(requestCount, requestType, requestID)
	case err := <-done:
		if err != nil {
			fmt.Printf("RPC call failed: %v\n", err)
			fmt.Println("Error connecting to Server. Trying to connect to Backup Server")
			SendRequest(1, requestType, requestID)
		} else {
			fmt.Println("RPC call succeeded, reply:", reply)
			if reply == WAIT {
				fmt.Println("Client", nodeID, "received WAIT from Leader ", message.LeaderID)
				receivedEnter := false
				// Start go function with timeout to wait for OK_ENTER from server
				// If no reply received within 5 seconds, request timeouts and proceeds to retry request
				ctx2 := sendMessageWithTimeout(client, &receivedEnter)
				select {
				case <-ctx2.Done():
					if !receivedEnter {
						fmt.Println("Request timed out")
						SendRequest(requestCount, requestType, requestID)
					}
				}
			} else if reply == OK_ENTER {
				fmt.Println("Entering Critical section")
				time.Sleep(2 * time.Second)
				SendRequest(1, RELEASE, requestID)
			} else if reply == OK_RELEASE {
				fmt.Println("Client", nodeID, "received OK_RELEASE from Leader ", message.LeaderID)
				requestCompleted = requestID
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

func sendMessageWithTimeout(client *rpc.Client, receivedEnter *bool) context.Context {
	// Declare a timeout for the request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	listenForServerMessages(receivedEnter)
	return ctx
}

func listenForServerMessages(receivedEnter *bool) {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("Server message failed", err)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleServerMessage(conn, receivedEnter)
	}
}

func handleServerMessage(conn net.Conn, receivedEnter *bool) {
	defer conn.Close()

	// Use bufio.NewReader to read from the connection
	reader := bufio.NewReader(conn)

	for {
		// ReadString reads until the first occurrence of the delimiter in the input
		message, err := reader.ReadString('\n') // '\n' is the line delimiter
		if err != nil {
			if err != io.EOF {
				fmt.Println("Error reading:", err)
			}
			break
		}

		// Process the received string message
		fmt.Printf("Received message: %s", message)
		if message == "OK_ENTER" {
			fmt.Println("Client", nodeID, "received OK_ENTER from Leader")
			// Some critical section function
			time.Sleep(2 * time.Second)
			*receivedEnter = true
			SendRequest(1, RELEASE, requestID)
		}
	}
}

// func SendRelease(client *rpc.Client) {
// 	var err error
// 	var reply MessageType
// }

// func RequestLock() {
// 	requestCount = 1
// 	requestID = 1
// 	SendRequest(requestCount, REQUEST, requestID)

// message.MessageType = RELEASE
// fmt.Println("Client", nodeID, "sent Release lock")
// SendRequest(client)

// // Queued requests
// time.Sleep(time.Second)

// message.Body = "REQUEST"
// message.RequestID += 1
// fmt.Println("Client", nodeID, "sent Request", message.RequestID, "for lock")
// SendRequest(client)

// time.Sleep(time.Second)

// message.Body = "RELEASE"
// fmt.Println("Client", nodeID, "sent Release lock")
// SendRequest(client)

// // New requests
// time.Sleep(4 * time.Second)

// message.Body = "REQUEST"
// message.RequestID += 1
// fmt.Println("Client", nodeID, "sent Request", message.RequestID, "for lock")
// SendRequest(client)

// time.Sleep(time.Second)

// message.Body = "RELEASE"
// fmt.Println("Client", nodeID, "sent Release lock")
// SendRequest(client)
// }

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
	nodeID, _ = strconv.Atoi(os.Getenv("NODE_ID"))
	// Wait for server goroutines to initialise
	// time.Sleep(7 * time.Second)

	// Start request for lock
	requestCount = 1
	requestID = 1
	requestCompleted = 0

	// Request for lock after
	for {
		go SendRequest(requestCount, REQUEST, requestID)
		for requestCompleted != requestID {
			time.Sleep(time.Second)
		}
		requestID++
		// Define the range for the timeout (e.g., between 1 and 5 seconds)
		minTimeout := 1
		maxTimeout := 5

		// Generate a random duration within the range
		randomDuration := time.Duration(rand.Intn(maxTimeout-minTimeout+1)+minTimeout) * time.Second
		time.Sleep(randomDuration)
	}
}
