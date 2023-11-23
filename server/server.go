package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

/*
	This server file is written in the perspective of a singular server object and defines its
	behaviors in the system
*/

// Request information for server-client communications
type Request struct {
	ClientID  int
	RequestID int
}

/*
This represents the Messages that will be sent between the server-client communications
*/
type ClientMessageType int

const (
	REQUEST    ClientMessageType = iota // for client to make a request to server
	RELEASE                             // for client to tell server that client is done with cs
	OK_RELEASE                          // for server to tell client that release is successful
	WAIT                                // for server to tell client to wait
	OK_ENTER                            // for server to signal to client to enter cs
)

type ClientMessage struct {
	ClientMessageType ClientMessageType
	LeaderID          int
	ClientID          int
	RequestID         int
}

/*
These are the properties the server has
*/
var (
	LeaderID int
	isLeader bool
	data     Data
	dataLock sync.Mutex
)

type Node int

// For Leader to handle requests from clients
func (n *Node) HandleClientRequest(arg *ClientMessage, reply *string) error {
	switch arg.ClientMessageType {
	case REQUEST:
	case RELEASE:
		// Handle case of RELEASE of lock
		if data.ActiveClient.ClientID == arg.ClientID && data.ActiveClient.RequestID == arg.RequestID {
			dataLock.Lock()
			data.ActiveClient.ClientID = 0
			data.ActiveClient.RequestID = 0
			if len(data.Queue) > 0 {
				data.ActiveClient.ClientID = data.Queue[0].ClientID
				data.ActiveClient.RequestID = data.Queue[0].RequestID
				data.Queue = data.Queue[1:]
			}
			dataLock.Unlock()
			*reply = "OK Release"
		} else {
			*reply = "Release not permitted without lock"
		}
	}


	if arg.ClientMessageType == REQUEST {
		// Handle case where there is a lock assigned to the client requesting again
		if data.ClientID == arg.ClientID && data.RequestID == arg.RequestID {
			// Serve re-request
			fmt.Println("Leader serving request from queue")
			*reply = "OK Rerequest"
		} else if data.ClientID != 0 {
			// Add client to queue
			dataLock.Lock()
			data.Queue = append(data.Queue, Request{arg.ClientID, arg.RequestID})
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
			data.ClientID = 0
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

/*
This represents the data that the distributed servers will replicate among each other
*/
type Data struct {
	ActiveClient Request
	// ClientID  int // represets which client is currently holding the lock
	// RequestID int
	Queue []Request
}

/*
This represents the Messages that will be sent between the server-server communications
*/
type ServerMessageType int

const (
	REPLICATE     ServerMessageType = iota // for leader server to replicate data to replica
	OK_REPLICATED                          // for slave to acknowledge replication successful
)

type ServerMessage struct {
	ServerMessageType ServerMessageType
	RequestID         int
	Data              Data
}

func (n *Node) HandleServerRequest(arg *ServerMessage, reply *string) error {
	switch arg.ServerMessageType {
	case REPLICATE:
	case OK_REPLICATED:

	}
	return nil
}

// func (n *Node) SendReplicatedDataToSlave() {
// 	slave, err := rpc.Dial("tcp", "slaveServerAddress")
// 	if err != nil {
// 			return fmt.Errorf("Dialing: %v", err)
// 	}
// }

// Set up Leader RPC gateway for communication, opening up message channel
func ServerListener() {
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

// For Leader to send back data for replication & aliveness
func (n *Node) ReplicateData(arg *Data, reply *Data) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	*reply = data

	fmt.Printf("Leader Data: %+v\n", data)
	return nil
}

// For Slave to poll for data update & handle simple failover procedure
func SlavePoller() {
	for {
		time.Sleep(5 * time.Second)

		if !isLeader {
			client, err := rpc.Dial("tcp", "node"+strconv.Itoa(LeaderID)+":8080")
			if err != nil {
				log.Println("Failed to connect to leader, becoming leader:", err)
				isLeader = true
				break
			}
			defer client.Close()

			var replicatedData Data
			dataLock.Lock()
			err = client.Call("Node.ReplicateData", nil, &replicatedData)

			if err != nil {
				log.Println(err)
				continue
			}
			data = replicatedData

			dataLock.Unlock()

			fmt.Printf("Slave Data: %+v\n", data)
		}
	}
}

func main() {
	nodeID := os.Getenv("NODE_ID")

	// Set node to isLeader if nodeID is 2
	isLeader := nodeID == "2"
	data.ActiveClient = Request{0, 0}

	go ServerListener()

	if !isLeader {
		go SlavePoller()
	}

	// Block main goroutine
	select {}
}
