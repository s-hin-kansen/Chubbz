package main

import (
	"encoding/json"
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
	Request           Request
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
func (n *Node) HandleClientMessage(arg *ClientMessage, reply *ClientMessage) error {
	switch arg.ClientMessageType {
	// TODO: This request should take in types of REQUEST and RELEASE
	// In doing so, it should also reply to the client either OK_RELEASE, WAIT, OK_ENTER
	case REQUEST:

		/*
			1. Get the request
			2. Check if someone else is an data.ActiveClient
				2.1. If not,change data.ActiveClient to client
				2.2.Add to queue
			3.Make a function call to the slave server to replicate data
				3.1.Wait for the slave server to reply OK
3.2. Slave server does not reply ok, might be down, don't think of it for now
			4. Reply to the client either
				4.1. OK_ENTER
				4.2. WAIT
		*/

		// Add client to queue
		dataLock.Lock()
		data.Queue = append(data.Queue, Request{arg.Request.ClientID, arg.Request.RequestID})
		dataLock.Unlock()
		*reply = "LOCKED"

		// Handle case where there is a lock assigned to the client requesting again (this handles case )
		if data.ActiveClient.ClientID == arg.Request.ClientID && data.ActiveClient.RequestID == arg.Request.RequestID {
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
	case RELEASE:
		// Handle case of RELEASE of lock for appropriate callers
		if data.ActiveClient.ClientID == arg.Request.ClientID && data.ActiveClient.RequestID == arg.Request.RequestID {
			dataLock.Lock()
			// Get next client in queue
			if len(data.Queue) > 0 {
				data.ActiveClient = data.Queue[0]
				// TODO: JON: Send OK_ENTER to the next active client (use SendClientMessage function)

				data.Queue = data.Queue[1:]
			} else {
				data.ActiveClient = Request{0, 0}
			}
			dataLock.Unlock()
			*reply = ClientMessage{OK_RELEASE, LeaderID, Request{0, 0}} // request here is redundant treat as a nil request
		}
	}
	// fmt.Printf("Leader Data: %+v\n", data)
	return nil
}

// TODO: create a way to send clients a message through serialising the JSON ClientMessage into bytes to be send
func (n *Node) SendClientMessage(message *ClientMessage) error {
	clientAddress := "client" + strconv.Itoa(message.Request.ClientID) + ":8080"

	conn, err := net.Dial("tcp", clientAddress)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Serialize the message using JSON
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// Send the serialized message
	_, err = conn.Write(messageBytes)
	if err != nil {
		return err
	}

	return nil
}

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
// TODO: add code to change beaviour of function based on idenity (slave / leader)
// if node is slave, the slave should take in the data from the args
// if node is leader the leader should take arg as nil, and reply with its own leader Data
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
			leader, err := rpc.Dial("tcp", "node"+strconv.Itoa(LeaderID)+":8080")
			if err != nil {
				log.Println("Failed to connect to leader, becoming leader:", err)
				isLeader = true
				break
			}
			defer leader.Close()

			dataLock.Lock()
			var replicatedData Data
			err = leader.Call("Node.ReplicateData", nil, &replicatedData)

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
