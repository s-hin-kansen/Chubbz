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
	Queue          []Request
	PreviousClient Request
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
	LeaderID  int
	isLeader  bool
	data      Data
	dataLock  sync.Mutex
	dead      bool
	nodeID    string
	sim       string
	clientNum int
)

type Node int

// For Leader to handle requests from clients
func (n *Node) HandleRequest(arg *ClientMessage, reply *ClientMessageType) error {
	if dead || !isLeader {
		time.Sleep(5 * time.Second)
		return nil
	}

	log.Printf("Leader received %+v: from Client %d\n", arg.ClientMessageType.String(), arg.Request.ClientID)
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

		req := arg.Request         // get the client request
		nonActive := Request{0, 0} // handle the trivial case when theres no active client at all
		if data.ActiveClient == nonActive {
			// set ActiveClient to the request
			data.ActiveClient = req

			// replicate leader data to slave
			n.LeaderReplicateDataToSlave()

			// reply to client OK_ENTER
			*reply = OK_ENTER

			// Simulation 3
			if sim == "3" {
				n.Dies()
				log.Printf("Node %s is now dead", nodeID)
			}
		} else {
			// add to queue
			data.Queue = append(data.Queue, req)

			// replicate leader data to slave
			n.LeaderReplicateDataToSlave()

			// reply to client WAIT
			*reply = WAIT
		}
	case RELEASE: // Handle case of RELEASE of lock for appropriate callers
		// This should simulate intermittent failure only once
		if sim == "2" && nodeID == "2" {
			dead = true
			isLeader = false
			LeaderID = 1
			log.Printf("Node %s is now dead", nodeID)
			time.Sleep(100 * time.Millisecond)
			dead = false
			isLeader = true
			LeaderID = 2
			log.Printf("Node %s is now alive", nodeID)
			sim = "0" // Fail only once
			time.Sleep(6 * time.Second)
		}

		if data.ActiveClient.ClientID == arg.Request.ClientID && data.ActiveClient.RequestID == arg.Request.RequestID {
			// Get next client in queue
			if len(data.Queue) > 0 {
				data.PreviousClient = data.ActiveClient // Save Previous request
				data.ActiveClient = data.Queue[0]
				data.Queue = data.Queue[1:]
			} else {
				data.PreviousClient = data.ActiveClient // Save Previous request
				data.ActiveClient = Request{0, 0}
			}
			// leader replicates data to slave
			n.LeaderReplicateDataToSlave()

			if sim == "4" && nodeID == "2" || sim == "5" && nodeID == "2" {
				n.Dies()
				log.Printf("Node %s is now dead", nodeID)

				if sim == "5" {
					go func() {
						time.Sleep(20 * time.Second)
						dead = false
						log.Printf("Node %s is now alive", nodeID)
					}()
				}
				return nil
			}
			*reply = OK_RELEASE
			log.Printf("Release of lock for Client %d", arg.Request.ClientID)

			if data.ActiveClient.ClientID != 0 {
				// Send OK_ENTER to the next active client (use SendClientMessage function)
				n.SendClientMessage(&ClientMessage{OK_ENTER, LeaderID, data.ActiveClient})
			}
		} else {
			// handle the case when leader goes down before sending OK_RELEASE to client and this new slave becomes leader,
			// and client sends RELEASE to this new leader, but this new leader does not have the client in its data.ActiveClient
			// trivially send the OK_RELEASE command to the lingering client

			*reply = OK_RELEASE
			log.Printf("Re-release of lock for Client %d", arg.Request.ClientID)

			// Fault tolerance for failing in between replications
			if data.PreviousClient.ClientID == arg.Request.ClientID && data.PreviousClient.RequestID == arg.Request.RequestID && data.ActiveClient.ClientID != 0 {
				n.SendClientMessage(&ClientMessage{OK_ENTER, LeaderID, data.ActiveClient})
			}
		}
	}
	return nil
}

// send clients a message through writing string to connection. main purpose is to send OK_ENTER to client
func (n *Node) SendClientMessage(message *ClientMessage) error {
	clientAddress := "client" + strconv.Itoa(message.Request.ClientID) + ":8080"
	log.Printf("Connecting to ---  %v", clientAddress)

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

// Util function for leader to call to replicate data to slave
// if the replication fails, we assume that the slave went down and we have no choice but to reply back to the client even after failed strong consistency replication
func (n *Node) LeaderReplicateDataToSlave() bool {
	var slaveID int
	if nodeID == "2" {
		slaveID = 1
	} else if nodeID == "1" {
		slaveID = 2
	}

	// dial slave
	slave, err1 := rpc.Dial("tcp", "node"+strconv.Itoa(slaveID)+":8080") // this function only works for original node2 leader replicating to node1 slave
	if err1 != nil {
		log.Println("Failed to connect to slave:", err1)
		return false
	}
	defer slave.Close()

	// replicate data to slave
	var reply Data

	err2 := slave.Call("Node.ReplicateData", &data, &reply) // leader passes in his own data, and ensures slave replies with the same data, this signals to leader the data is indeed replicated

	if err2 != nil {
		log.Println("Replication failed,", err2) // Should check if the node is dead for replication
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
	if reply.PreviousClient != data.PreviousClient {
		log.Println("Slave data not replicated correctly")
		return false
	}

	log.Printf("Slave data replicated correctly in LeaderReplicateToSalve func")
	return true
}

// For Leader to send back data for replication & aliveness
// added code to change beaviour of function based on idenity (slave / leader)
// if node is slave, the slave should take in the data from the args
// if node is leader the leader should take arg as nil, and reply with its own leader Data
func (n *Node) ReplicateData(arg *Data, reply *Data) error {
	if dead {
		return fmt.Errorf("Node %s is dead", nodeID)
	}

	if isLeader {
		// ignore the input nil Data of external function call by slave
		// reply the slave of my own leader data
		*reply = data
	}

	if !isLeader {
		// take in the input Data of external function call by leader
		// reply the leader of my updated slave data
		data = *arg
		*reply = data
	}
	fmt.Printf("Replicated Data: %+v\n", data)

	return nil
}

// Slave ONLY: Poll for data update & handle simple failover procedure
func ServerPoller() {
	for {
		time.Sleep(5 * time.Second)

		// Don't poll if it is a leader or not dead
		if isLeader || dead {
			continue
		} else if !isLeader {
			leader, err := rpc.Dial("tcp", "node"+strconv.Itoa(LeaderID)+":8080")
			defer leader.Close()

			// If unable to connect
			if err != nil {
				log.Println("Failed to connect to leader", err)
			}

			// Check if leader is dead through a specific call
			var leaderDead bool
			err = leader.Call("Node.AreYouDead", "", &leaderDead)
			if err != nil {
				log.Println(err)
				continue
			}

			// Check if other leader is dead
			if leaderDead {
				LeaderID = 1
				isLeader = true

				// Announcing to all clients that it won the election
				// Since we are planning for 2 clients only, there will no announcement to other server that it is the leader
				// NOTE: Change i to the number of clients whenever necessary
				for i := 1; i <= clientNum; i++ {
					client, err := net.Dial("tcp", "client"+strconv.Itoa(i)+":8081")
					_, err = client.Write([]byte(nodeID))
					if err != nil {
						log.Println("Failed to connect to client:", err)
						continue
					}
					defer client.Close()
					log.Printf("Announced to Client %d that I am the new leader", i)
				}
				break
			}

			var replicatedData Data
			err = leader.Call("Node.ReplicateData", &data, &replicatedData)

			if err != nil {
				log.Println(err)
				continue
			}
			data = replicatedData
		}
	}
}

func main() {
	nodeID = os.Getenv("NODE_ID")
	sim = os.Getenv("SIM_NO")
	clientNumStr := os.Getenv("CLIENT_NUM")

	var err error
	clientNum, err = strconv.Atoi(clientNumStr)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Node ID: %v, initialised\n", nodeID)

	// Set node to isLeader if nodeID is 2
	isLeader = nodeID == "2"
	LeaderID = 2
	data.ActiveClient = Request{0, 0}
	dead = false

	go ServerListener()

	go ServerPoller()

	// Block main goroutine
	select {}
}

// Death simulation
func (n *Node) Dies() {
	// All simulations are for node 2, so ignore if node 1
	if nodeID == "1" {
		return
	}

	dead = true
	isLeader = false
	if nodeID == "0" {
		LeaderID = 1
	} else if nodeID == "1" {
		LeaderID = 0
	}
	log.Printf("Node %s is now dead", nodeID)
}

// Checking if the node is dead
func (n *Node) AreYouDead(arg *string, reply *bool) error {
	*reply = dead
	return nil
}
