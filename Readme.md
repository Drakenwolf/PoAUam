# P2P Blockchain Implementation in Go
This project is a simple implementation of a blockchain in a peer-to-peer (P2P) network using the Go programming language and the libp2p library.

## Overview
The blockchain is a chain of blocks, where each block contains data. In this case, the data represents a product with an ID, location, and owner. The blockchain is stored in a slice of Block structs. The network of nodes is managed using the libp2p library, which allows for P2P communication.

## Data Structures
### Block
Each block in the blockchain is represented by a Block struct:


type Block struct {
	ha        host.Host
	Index     int
	Timestamp string
	ProductID string
	Location  string
	Owner     string
	Hash      string
	PrevHash  string
	Validator string
}
### Proposal
A Proposal struct is used to represent a proposal for a new node to be added to the network:


type Proposal struct {
	NodeID string
	Votes  int
}
## Functions
### makeBasicHost
The makeBasicHost function is used to create a basic libp2p host. A libp2p host is a node in the network that can communicate with other nodes. The function takes a listen port, a boolean indicating whether to use secio (a secure transport), and a seed for generating a random ID for the host.

### handleStream
The handleStream function is a stream handler that is called when a new stream is opened between hosts in the network. It reads and writes data to and from the stream.

### readData
The readData function reads data from a stream. It unmarshals the received data into a slice of Block structs and replaces the current blockchain with the received one if it's longer.

### writeData
The writeData function writes the current blockchain to a stream every 5 seconds. It also reads input from the user to add new blocks to the blockchain.

### isBlockValid
The isBlockValid function checks if a block is valid by comparing it to the previous block. A block is considered valid if its index is one more than the previous block's index, its PrevHash field is equal to the previous block's Hash field, and its hash is correctly calculated.

### isNodeAuthorized
The isNodeAuthorized function checks if a node is authorized by comparing its ID to the IDs in the AuthorizedNodes slice.

### calculateHash
The calculateHash function calculates the SHA-256 hash of a block.

### generateBlock
The generateBlock function generates a new block based on the previous block and the provided data.

## Usage
### To run a node, use the following command:


go run main.go -l <listen_port>
Replace <listen_port> with the port number you want the node to listen on.

### To connect a node to another node, use the following command:


go run main.go -l <listen_port> -d <target_address>
Replace <target_address> with the address of the node you want to connect to.