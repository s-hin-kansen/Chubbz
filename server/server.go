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

func (cmt ClientMessageType) String() string {
	switch cmt {
	case REQUEST:
		return "REQUEST"
	case RELEASE:
		return "RELEASE"
	case OK_RELEASE:
		return "OK_RELEASE"
	case WAIT:
		return "WAIT"
	case OK_ENTER:
		return "OK_ENTER"
	default:
		return "Unknown"
	}
}

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
func (n *Node) HandleRequest(arg *ClientMessage, reply *ClientMessageType) error {
	fmt.Printf("Leader received %+v: from Client %d\n", arg.ClientMessageType.String(), arg.Request.ClientID)
	switch arg.ClientMessageType {
	// This request should take in types of REQUEST and RELEASE
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

		req := arg.Request // get the client request

		nonActive := Request{0, 0} // handle the trivial case when theres no active client at all
		if data.ActiveClient == nonActive {
			// set ActiveClient to the request
			dataLock.Lock()
			data.ActiveClient = req
			dataLock.Unlock()
			// replicate leader data to slave
			n.LeaderReplicateDataToSlave() // QUESTION: do we need to lock and unlock mutex lock during replication?
			// reply to client OK_ENTER - TODO: NOTE client side currently does not handle this, can possibly ask client side to help upgrade, or else for now we will send 2x OK_ENTER	in 2 diff ways since the client does support the PATCH below
			*reply = OK_ENTER
			// PATCH we hope to rmv this send OK_ENTER by the send one way msg way. but note still need to use the one way msg function upon dequeuing active client and telling next client in queue to OK_ENTER
			//n.SendClientMessage(&ClientMessage{OK_ENTER, LeaderID, data.ActiveClient})
		} else {
			// add to queue
			dataLock.Lock()
			data.Queue = append(data.Queue, req)
			dataLock.Unlock()
			// replicate leader data to slave
			n.LeaderReplicateDataToSlave() // QUESTION: do we need to lock and unlock mutex lock during replication?
			// reply to client WAIT
			*reply = WAIT
		}

		// // Add client to queue
		// dataLock.Lock()
		// data.Queue = append(data.Queue, Request{arg.Request.ClientID, arg.Request.RequestID})
		// dataLock.Unlock()
		// *reply = "LOCKED"

		// // Handle case where there is a lock assigned to the client requesting again (this handles case )
		// if data.ActiveClient.ClientID == arg.Request.ClientID && data.ActiveClient.RequestID == arg.Request.RequestID {
		// 	// Serve re-request
		// 	fmt.Println("Leader serving request from queue")
		// 	*reply = "OK Rerequest"
		// } else if data.ClientID != 0 {
		// 	// Add client to queue
		// 	dataLock.Lock()
		// 	data.Queue = append(data.Queue, Request{arg.ClientID, arg.RequestID})
		// 	dataLock.Unlock()
		// 	*reply = "LOCKED"
		// } else {
		// 	// New request
		// 	fmt.Println("leader Received request")
		// 	dataLock.Lock()
		// 	data.ClientID = arg.ClientID
		// 	data.RequestID = arg.RequestID
		// 	dataLock.Unlock()
		// 	*reply = "OK New request"
		// }
	case RELEASE:
		// Handle case of RELEASE of lock for appropriate callers
		// fmt.Println(data.ActiveClient, arg.Request)
		// dataLock.Lock()
		if data.ActiveClient.ClientID == arg.Request.ClientID && data.ActiveClient.RequestID == arg.Request.RequestID {
			// Get next client in queue
			if len(data.Queue) > 0 {
				data.ActiveClient = data.Queue[0]
				data.Queue = data.Queue[1:]
				// Send OK_ENTER to the next active client (use SendClientMessage function)
				defer n.SendClientMessage(&ClientMessage{OK_ENTER, LeaderID, data.ActiveClient})
			} else {
				data.ActiveClient = Request{0, 0}
			}
			// leader replicates data to slave
			n.LeaderReplicateDataToSlave() // QUESTION: do we need to lock and unlock mutex lock during replication?
			log.Printf("Replicated data to slave")
			// dataLock.Unlock()
			*reply = OK_RELEASE
			log.Printf("Release of lock for Client %d", arg.Request.ClientID)
			// dataLock.Unlock()
		}
	}
	// fmt.Printf("Leader Data: %+v\n", data)
	return nil
}

// send clients a message through writing string to connection. main purpose is to send OK_ENTER to client
func (n *Node) SendClientMessage(message *ClientMessage) error {
	clientAddress := "client" + strconv.Itoa(message.Request.ClientID) + ":8080"
	log.Printf("Connecting to ---  %v", clientAddress)

	for {
		conn, err := net.Dial("tcp", clientAddress)
		if err != nil {
			log.Println(err)
			// return err
		}
		if err == nil {
			defer conn.Close()
			_, err = conn.Write([]byte(message.ClientMessageType.String()))
			if err != nil {
				return err
			}
			log.Printf("Sent OK_ENTER to Client %d", data.ActiveClient.ClientID)
			return nil
		}
		time.Sleep(1 * time.Second)
	}
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

// util function for leader to call to replicate data to slave
// if the replication fails, we assume that the slave went down and we have no choice but to reply back to the client even after failed strong consistency replication
func (n *Node) LeaderReplicateDataToSlave() bool {
	// dial slave
	slave, err := rpc.Dial("tcp", "node"+strconv.Itoa(1)+":8080") // this function only works for original node2 leader replicating to node1 slave
	if err != nil {
		log.Println("Failed to connect to slave:", err)
		return false
	}
	defer slave.Close()

	// replicate data to slave
	dataLock.Lock()
	var reply Data
	err = slave.Call("Node.ReplicateData", &data, &reply) // leader passes in his own data, and ensures slave replies with the same data, this signals to leader the data is indeed replicated
	if err != nil {
		log.Println(err)
		return false
	}

	// check slaves reply Data to be identical to my leader Data
	if reply.ActiveClient != data.ActiveClient {
		log.Println("Slave data not replicated correctly")
		return false
	}
	if len(reply.Queue) != len(data.Queue) {
		log.Println("Slave data not replicated correctly")
		return false
	} else {
		for i, v := range data.Queue {
			if v != reply.Queue[i] {
				log.Println("Slave data not replicated correctly")
				return false
			}
		}
	}
	dataLock.Unlock()
	log.Printf("Slave data replicated correctly in LeaderReplicateToSalve func")
	return true
}

// For Leader to send back data for replication & aliveness
// added code to change beaviour of function based on idenity (slave / leader)
// if node is slave, the slave should take in the data from the args
// if node is leader the leader should take arg as nil, and reply with its own leader Data
func (n *Node) ReplicateData(arg *Data, reply *Data) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	if isLeader {
		// ignore the input nil Data of external function call by slave
		// reply the slave of my own leader data
		*reply = data
		// fmt.Printf("Leader Data: %+v\n", data)
	}

	if !isLeader {
		// take in the input Data of external function call by leader
		// reply the leader of my updated slave data
		data = *arg
		*reply = data
		fmt.Printf("Replica Data: %+v\n", data)
		fmt.Printf("Master Data: %+v\n", *arg)
	}

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
			err = leader.Call("Node.ReplicateData", &data, &replicatedData)

			if err != nil {
				log.Println(err)
				continue
			}
			data = replicatedData

			dataLock.Unlock()

			// fmt.Printf("Slave Data: %+v\n", data)
		}
	}
}

func main() {
	nodeID := os.Getenv("NODE_ID")
	fmt.Printf("Node ID: %v, initialised\n", nodeID)

	// Set node to isLeader if nodeID is 2
	isLeader := nodeID == "2"
	LeaderID = 2
	data.ActiveClient = Request{0, 0}

	go ServerListener()

	if !isLeader {
		go SlavePoller()
	}

	// Block main goroutine
	select {}
}
