package main

import (
	"fmt"
	"log"
	"net/rpc"
	"os"
	"time"
)

type Message struct {
	ClientID  string
	RequestID int
	Body      string
}

var (
	message Message
)

func SendRequest(client *rpc.Client) {
	var err error
	var reply string
	err = client.Call("Node.HandleRequest", &message, &reply)
	if err != nil {
		log.Fatal(err)

		log.Println("Error connecting to Server. Trying to connect to Backup Server")
		go contactBackup()
	}
	// fmt.Println("Received:", reply)
	// if reply == "LOCKED" {
	// 	// TODO: retry indefinitely?

	// 	// IGNORE: Below is for debug of RequestID
	// 	time.Sleep(6 * time.Second)
	// 	err = client.Call("Node.HandleRequest", &message, &reply)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	fmt.Println("Received:", reply)
	// 	// END debug
}

func RequestLock(client *rpc.Client, nodeID string) {
	message.ClientID = nodeID

	message.Body = "REQUEST"
	message.RequestID = 1
	fmt.Println("Client", nodeID, "sent Request", message.RequestID, "for lock")
	SendRequest(client)

	// Some critical section function
	time.Sleep(time.Second)

	message.Body = "RELEASE"
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

func contactServer() {
	nodeID := os.Getenv("NODE_ID")
	leaderAddress := os.Getenv("LEADER_ADDRESS") // Assume the format is "node2:8080"
	client, err := rpc.Dial("tcp", leaderAddress)
	if err != nil {
		log.Fatal("Failed to connect to leader:", err)
	}
	defer client.Close()
	fmt.Println("Client", nodeID, "connected to server")
	go RequestLock(client, nodeID)
	time.Sleep(30 * time.Second)
}

func contactBackup() {
	nodeID := os.Getenv("NODE_ID")
	backupAddress := os.Getenv("BACKUP_ADDRESS") // Assume the format is "node2:8080"
	client, err := rpc.Dial("tcp", backupAddress)
	if err != nil {
		log.Fatal("Failed to connect to backup  server:", err)
	}
	defer client.Close()
	fmt.Println("Client", nodeID, "connected to backup server")
	go RequestLock(client, nodeID)
	time.Sleep(30 * time.Second)
}

func main() {
	time.Sleep(7 * time.Second)
	go contactServer()
	time.Sleep(50 * time.Second)
}
