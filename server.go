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

type Data struct {
	ClientID int
	Queue    []int
}

type Node int

var (
	isLeader bool
	data     Data
	dataLock sync.Mutex
)

func (n *Node) HandleRequest(arg *string, reply *string) error {
	if *arg == "REQUEST" {
		fmt.Println("leader Received request")
		dataLock.Lock()
		data.ClientID = 1
		dataLock.Unlock()
		*reply = "OK"
	} else if *arg == "RELEASE" {
		dataLock.Lock()
		data.ClientID = 0
		dataLock.Unlock()
		*reply = "OK"
	}
	// fmt.Printf("Leader Data: %+v\n", data)
	return nil
}

func (n *Node) ReplicateData(arg *Data, reply *Data) error {
	dataLock.Lock()
	defer dataLock.Unlock()

	*reply = data
	data = *arg

	fmt.Printf("Leader Data: %+v\n", data)
	return nil
}

func main() {
	nodeID := os.Getenv("NODE_ID")
	isLeader := nodeID == "2"

	if isLeader {
		go leaderListener()
	} else {
		go slavePoller()
	}

	// Block main goroutine
	select {}
}

func leaderListener() {
	node := new(Node)
	rpc.Register(node)t

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
			err = client.Call("Node.ReplicateData", &data, &reply)
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
