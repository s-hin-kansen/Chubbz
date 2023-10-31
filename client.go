package main

import (
	"fmt"
	"log"
	"net/rpc"
	"os"
	"time"
)

// type Node int

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
	}
	fmt.Println("Received:", reply)
	if reply == "LOCKED" {
		// TODO: retry indefinitely?

		// IGNORE: Below is for debug of RequestID
		time.Sleep(6 * time.Second)
		err = client.Call("Node.HandleRequest", &message, &reply)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Received:", reply)
		// END debug
	}
}

func RequestLock(client *rpc.Client, nodeID string) {
	// nodeID := os.Getenv("NODE_ID")
	message.ClientID = nodeID

	message.Body = "REQUEST"
	message.RequestID = 1
	fmt.Println("Client", nodeID,"sent Request", message.RequestID,"for lock")
	SendRequest(client)
	// err = client.Call("Node.HandleRequest", &message, &reply)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("Received:", reply)

	// Some critical section function
	time.Sleep(time.Second)

	message.Body = "RELEASE"
	fmt.Println("Client", nodeID,"sent Release lock")
	SendRequest(client)

	// err = client.Call("Node.HandleRequest", &message, &reply)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("Received:", reply)

	time.Sleep(8 * time.Second)

	message.Body = "REQUEST"
	message.RequestID += 1
	fmt.Println("Client", nodeID,"sent Request", message.RequestID,"for lock")
	SendRequest(client)

	time.Sleep(time.Second)

	message.Body = "RELEASE"
	fmt.Println("Client", nodeID,"sent Release lock")
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
	fmt.Println("Client", nodeID,"connected to server")
	go RequestLock(client, nodeID)
	time.Sleep(30 * time.Second)
}

func main() {
	time.Sleep(7 * time.Second)
	go contactServer()
	time.Sleep(50 * time.Second)
	// leaderAddress := os.Getenv("LEADER_ADDRESS") // Assume the format is "node2:8080"

	// nodeID := os.Getenv("NODE_ID")
	// client, err := rpc.Dial("tcp", leaderAddress)
	// if err != nil {
	// 	log.Fatal("Failed to connect to leader:", err)
	// }
	// defer client.Close()

	// request := "REQUEST"
	// var reply string
	// fmt.Println("Client sent Request for lock")
	// err = client.Call("Node.HandleRequest", &request, &reply)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("Received:", reply)

	// time.Sleep(5 * time.Second)

	// release := "RELEASE"
	// fmt.Println("Client sent Release lock")
	// err = client.Call("Node.HandleRequest", &release, &reply)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("Received:", reply)
}
