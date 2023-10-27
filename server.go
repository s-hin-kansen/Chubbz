package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
)

// Essentially keep track of who has the lock & next in queue
type Data struct {
	ClientID int
	Queue    []int
}

var (
	isLeader bool
	data     Data
	dataLock sync.Mutex
)

type Node int

// For Leader to handle requests from clients
func (n *Node) HandleRequest(arg *string, reply *string) error {
	if *arg == "REQUEST" {
		// TODO: See how this can take in some client ID. Is it possible to get it from the RPC connection or only through the string?

		// Handle case where there is a lock assigned
		if data.ClientID != 0 {
			// Add client to queue
			dataLock.Lock()
			data.Queue = append(data.Queue, 1)
			dataLock.Unlock()
		}

		fmt.Println("leader Received request")
		dataLock.Lock()
		data.ClientID = 1
		dataLock.Unlock()
		*reply = "OK"
	} else if *arg == "RELEASE" {
		// Handle case of RELEASE of lock
		dataLock.Lock()
		data.ClientID = 0
		if len(data.Queue) > 0 {
			data.ClientID = data.Queue[0]
			data.Queue = data.Queue[1:]
		}
		dataLock.Unlock()
		*reply = "OK"
	}
	// fmt.Printf("Leader Data: %+v\n", data)
	return nil
}

// For Leader to send back data for replication & aliveness
func (n *Node) ReplicateData(arg *Data, reply *Data) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	*reply = data

	fmt.Printf("Leader Data: %+v\n", data)
	return nil
}

// Set up Leader RPC gateway for comms
func leaderListener() {
	node := new(Node)
	rpc.Register(node)

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}

// For Slave to poll for data update & handle simple failover procedure
func slavePoller() {
	for {
		time.Sleep(5 * time.Second)

		if !isLeader {
			client, err := rpc.Dial("tcp", "node2:8080")
			if err != nil {
				log.Println("Failed to connect to leader, becoming leader:", err)
				isLeader = true
				go leaderListener()
				continue
			}
			defer client.Close()

			var reply Data
			dataLock.Lock()
			err = client.Call("Node.ReplicateData", &struct{}{}, &reply)
			dataLock.Unlock()
			if err != nil {
				log.Println(err)
				continue
			}

			dataLock.Lock()
			data = reply
			dataLock.Unlock()

			fmt.Printf("Slave Data: %+v\n", data)
		}
	}
}

func main() {
	nodeID := os.Getenv("NODE_ID")

	// Set node to isLeader if nodeID is 2
	isLeader := nodeID == "2"

	if isLeader {
		go leaderListener()
	} else {
		go slavePoller()
	}

	// Block main goroutine
	select {}
}
