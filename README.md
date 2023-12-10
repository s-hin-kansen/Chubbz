# Chubbz
50.041 Distributed Systems and Computing

Team 8:
1. Foo Chuan Shao, [chuanshaof](https://github.com/chuanshaof)
2. Jon-Taylor Lim [JTZ18](https://github.com/JTZ18)
3. Sim Wang Lin, [99wanglin](https://github.com/99wanglin)
4. Shaun Hin, [s-hin-kansen](https://github.com/s-hin-kansen)

# Overview
The scope of this project, Chubbz, is intended to be a recreation of [Google's Chubby Lock Service](https://static.googleusercontent.com/media/research.google.com/en//archive/chubby-osdi06.pdf) using the language of Go. 

Chubbz utilizes several distributed systems concepts to ensure that the system is safe and consistent upon usages. The implementation includes 2 main files, `server.go` and `client.go`. The client will make requests to the server to obtain or release locks. The server will handle the requests accordingly to the implementation defined.

The implementation of Chubbz will only cover several simulations that are listed below.

## Simulations
Sequence diagrams of the simulations can be found in the folder `sequence_diagram`. The below are a list of the scenarios that we have simulated and tested.
1. Normal & no faults
2. Intermittent down upon client requests, but comes back up soon after
3. Permanent down after granting FIRST lock
4. Permanent down AFTER receiving release & data replication but before replying RELEASE OK
5. "Permanent" down, comes back alive after Backup takes control

## How to run the simulations
1. Ensure that docker is installed locally and is running
2. Copy the contents of `docker-compose-sim<simulation number>.yaml` into `docker-compose.yaml`
3. Run `docker-compose up `


# File Documentation
This will provide a detailed documentation of the 2 files that we have, namely `server.go` and `client.go`.

# server.go
## Overview
This Go package implements the functionality of the server handling and giving out locks.

## Struct Information
### Request
- ClientID: Requester's ID
- RequestID: To ensure that requests from the same client are differentiated

### ClientMessageType
This is an enum of the types of messages that will be passed around between Clients and Servers. There is a function to map this enum to its string called `String` as well.

### ClientMessage
- ClientMessageType: Enum from above
- LeaderID: Meant for leader to pass back ID for the client to know who is the new leader
- Request: Request if necessary

### Data
- ActiveClient: Stores which client/request has the current lock
- Queue: List to ensure sequential consistency
- PreviousClient: Implemented for fault tolerance purposes

### ServerMessageType
This is an enum of types of messages to be passed around between servers. 

### ServerMessage
- ServerMessageType: Enum from above
- RequestID: Ensure that the data is not doubled
- Data: Data to be replicated from the other server

### Vars
These are the set of local variables that each servers will have.

## Methods
### HandleRequest
This method will be called by the client through a RPC function call. This method will handle the various client requests and handle it as accordingly. It will then reply the client on the next course of action.

### SendClientMessage
Sends client a one-way message. In this case, it is used to send messages of type "OK_ENTER" in the specific case where a previous lock has been released and the client is next in the queue.

### ServerListener
Sets up Leader RPC gateway for communication from both clients and servers.

### LeaderReplicateDataToSlave
This method is called to replicate data and ensure strong consistency. 

### ReplicateData
Replicate data through the use of RPC calls. This is a two-way communication method.

### ServerPoller
Helps for servers to poll for updates and handle failover procedures. This is similar to a heartbeat check used by the replicas, in this case, the backup.

### Dies
Helper function to simulate a server dying. This is only used for server 2 in all our simulations.

### AreYouDead
As we do not actually close off servers, this helper function helps with the heartbeat system to simulate a client is dead.


# client.go
## Overview
This Go package implements the functionality of how a client is expected to behave when making requests to the server. In this portion of the documentation, we intentionally omit information that has been repeated in `server.go` and will only cover differences that have been found.

## Struct Information
### Vars
These are the set of local variables that each client will have. It stores its own requests information and the leader it should be sending its information to.

## Methods
### SendRequest
This method is called for the client to send messages to the server through the help of RPC function calls, mostly to `HandleRequest`. This method will handle cases such as retry mechanism upon no-responses and such.

### listenForServerMessages
Listen for one-way messages from the server. This is used to receive messages of "OK_ENTER" after waiting. 

### handleServerMessage
Helper function for the above. Also, will send a release request to simulate and move the system forward.

### listenForLeaderElection
Listen for one-way messages from the server to get updated on possible faults from the server side.
