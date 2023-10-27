package main

import (
	"fmt"
	"log"
	"net/rpc"
	"os"
	"time"
)

func main() {
	time.Sleep(5 * time.Second)
	leaderAddress := os.Getenv("LEADER_ADDRESS") // Assume the format is "node2:8080"

	client, err := rpc.Dial("tcp", leaderAddress)
	if err != nil {
		log.Fatal("Failed to connect to leader:", err)
	}
	defer client.Close()

	request := "REQUEST"
	var reply string
	fmt.Println("Client sent Request for lock")
	err = client.Call("Node.HandleRequest", &request, &reply)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Received:", reply)

	time.Sleep(5 * time.Second)

	release := "RELEASE"
	fmt.Println("Client sent Release lock")
	err = client.Call("Node.HandleRequest", &release, &reply)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Received:", reply)
}
