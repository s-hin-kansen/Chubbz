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
type IDs struct {
	ClientID  string
	RequestID int
}

type Data struct {
	ClientID  string
	RequestID int
	Queue     []IDs
}

type Message struct {
	ClientID  string
	RequestID int
	Body      string
}

var (
	isLeader bool
	data     Data
	dataLock sync.Mutex
)

type Node int

// For Leader to handle requests from clients
func (n *Node) HandleRequest(arg *Message, reply *string) error {
	if arg.Body == "REQUEST" {
		// TODO: See how this can take in some client ID. Is it possible to get it from the RPC connection or only through the string?

		// Handle case where there is a lock assigned
		if data.ClientID == arg.ClientID && data.RequestID == arg.RequestID {
			// Serve re-request
			fmt.Println("Leader serving request from queue")
			*reply = "OK Rerequest"
		} else if data.ClientID != "0" {
			// Add client to queue
			dataLock.Lock()
			data.Queue = append(data.Queue, IDs{arg.ClientID, arg.RequestID})
			dataLock.Unlock()
			*reply = "LOCKED" 
		} else {
			// New request
			fmt.Println("leader Received request")
			dataLock.Lock()
			data.ClientID = arg.ClientID
			data.RequestID = arg.RequestID
			dataLock.Unlock()
			*reply = "OK New request"
		}
	} else if arg.Body == "RELEASE" {
		// Handle case of RELEASE of lock
		if data.ClientID == arg.ClientID && data.RequestID == arg.RequestID {
			dataLock.Lock()
			data.ClientID = "0"
			data.RequestID = 0
			if len(data.Queue) > 0 {
				data.ClientID = data.Queue[0].ClientID
				data.RequestID = data.Queue[0].RequestID
				data.Queue = data.Queue[1:]
			}
			dataLock.Unlock()
			*reply = "OK Release"
		} else {
			*reply = "Release not permitted without lock"
		}
		
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
	data.ClientID = "0"
	data.RequestID = 0

	if isLeader {
		go leaderListener()
	} else {
		go slavePoller()
	}

	// Block main goroutine
	select {}
}
