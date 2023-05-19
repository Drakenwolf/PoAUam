package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

type Block struct {
	ha        host.Host // Add this line
	Index     int
	Timestamp string
	ProductID string
	Location  string
	Owner     string
	Hash      string
	PrevHash  string
	Validator string
}

type Proposal struct {
	NodeID string
	Votes  int
}

var (
	Blockchain      []Block
	AuthorizedNodes []string
	Proposals       []Proposal
	mutex           = &sync.Mutex{}
	ha              host.Host // Add this line
)

func InitializeAuthorizedNodes(nodes []string) {
	AuthorizedNodes = nodes
}

func AddNode(nodeID string) {
	mutex.Lock()
	AuthorizedNodes = append(AuthorizedNodes, nodeID)
	mutex.Unlock()
	fmt.Printf("Node %s added to the list of authorized nodes.\n", nodeID)
}

func makeBasicHost(listenPort int, secio bool, randseed int64) (host.Host, error) {

	var r io.Reader
	if randseed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(randseed))
	}

	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)),
		libp2p.Identity(priv),
	}

	basicHost, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}

	hostAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ipfs/%s", basicHost.ID().Pretty()))

	addrs := basicHost.Addrs()
	var addr multiaddr.Multiaddr
	for _, i := range addrs {
		if strings.HasPrefix(i.String(), "/ip4") {
			addr = i
			break
		}
	}
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("I am %s\n", fullAddr)
	if secio {
		log.Printf("Now run \"go run main.go -l %d -d %s -secio\" on a different terminal\n", listenPort+1, fullAddr)
	} else {
		log.Printf("Now run \"go run main.go -l %d -d %s\" on a different terminal\n", listenPort+1, fullAddr)
	}

	return basicHost, nil
}

func handleStream(stream network.Stream) {

	log.Println("Got a new stream from peer:", stream.Conn().RemotePeer())

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)

}

func readData(rw *bufio.ReadWriter) {

	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// log.Println("Received data:", str)
			if strings.HasPrefix(str, "proposenode,") {
				nodeID := strings.TrimPrefix(str, "proposenode,")
				// Add the proposal to the list of proposals
				Proposals = append(Proposals, Proposal{NodeID: nodeID, Votes: 0})
				fmt.Printf("Node %s proposed for addition.\n", nodeID)
				continue
			}

			chain := make([]Block, 0)
			if err := json.Unmarshal([]byte(str), &chain); err != nil {
				log.Fatal(err)
			}

			mutex.Lock()
			if len(chain) > len(Blockchain) {
				Blockchain = chain
				bytes, err := json.MarshalIndent(Blockchain, "", "  ")
				if err != nil {
					log.Fatal(err)
				}
				// Green console color: 	\x1b[32m
				// Reset console color: 	\x1b[0m
				fmt.Printf("\x1b[32m%s\x1b[0m> ", string(bytes))

				fmt.Printf("%v\n", AuthorizedNodes)

			}
			mutex.Unlock()
		}
	}
}

// add data validation to prevent injection attacks

func writeData(rw *bufio.ReadWriter) {

	go func() {
		for {
			time.Sleep(5 * time.Second)
			mutex.Lock()
			bytes, err := json.Marshal(Blockchain)
			if err != nil {
				log.Println(err)
			}

			rw.WriteString(fmt.Sprintf("%s\n", string(bytes)))

			// log.Println("Sent data:", string(bytes))

			rw.Flush()
			mutex.Unlock()
		}
	}()

	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		sendData = strings.Replace(sendData, "\n", "", -1)

		// hech if the command has the proposed node:

		if strings.HasPrefix(sendData, "proposenode,") {
			nodeID := strings.TrimPrefix(sendData, "proposenode,")
			// Add the proposal to the list of proposals
			Proposals = append(Proposals, Proposal{NodeID: nodeID, Votes: 1})
			// Send the proposal to all other nodes
			rw.WriteString(fmt.Sprintf("proposenode,%s\n", nodeID))
			rw.Flush()
			continue
		}

		if strings.HasPrefix(sendData, "vote,") {
			parts := strings.Split(sendData, ",")
			if len(parts) != 3 {
				continue
			}
			nodeID, vote := parts[1], parts[2]
			// Count the vote
			for i, proposal := range Proposals {
				if proposal.NodeID == nodeID {
					if vote == "yes" {
						proposal.Votes++
						Proposals[i] = proposal
						// If a majority of nodes voted 'yes', add the new node
						if proposal.Votes > len(AuthorizedNodes)/2 {
							AddNode(nodeID)
							// Remove the proposal after the node is added
							Proposals = append(Proposals[:i], Proposals[i+1:]...)
						}
					}
					break
				}
			}
			continue
		}

		// Assume the sendData in format: productID,location,owner,validator
		data := strings.Split(sendData, ",")
		if len(data) != 3 {
			log.Println("Invalid input. Please enter in the format: productID,location,owner")
			continue
		}
		productID, location, owner := data[0], data[1], data[2]
		validator := ha.ID().Pretty() // Use the host's ID as the validator

		newBlock := generateBlock(Blockchain[len(Blockchain)-1], productID, location, owner, validator)
		if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
			mutex.Lock()
			Blockchain = append(Blockchain, newBlock)
			mutex.Unlock()
		} else {
			log.Println("New block is not valid")
		}

		bytes, err := json.Marshal(Blockchain)
		if err != nil {
			log.Println(err)
		}

		spew.Dump(Blockchain)

		mutex.Lock()
		rw.WriteString(fmt.Sprintf("%s\n", string(bytes)))
		rw.Flush()
		mutex.Unlock()
	}
}

func main() {

	t := time.Now()
	genesisBlock := Block{
		Index:     0,
		Timestamp: t.String(),
		ProductID: "genesis",
		Location:  "genesis",
		Owner:     "genesis",
		Hash:      "",
		PrevHash:  "",
		Validator: "genesis",
	}

	Blockchain = append(Blockchain, genesisBlock)

	listenF := flag.Int("l", 0, "wait for incoming connections")
	target := flag.String("d", "", "target peer to dial")
	secio := flag.Bool("secio", false, "enable secio")
	seed := flag.Int64("seed", 0, "set random seed for id generation")
	flag.Parse()

	if *listenF == 0 {
		log.Fatal("Please provide a port to bind on with -l")
	}
	var err error
	ha, err = makeBasicHost(*listenF, *secio, *seed)
	if err != nil {
		log.Fatal(err)
	}

	if len(AuthorizedNodes) == 0 {
		InitializeAuthorizedNodes([]string{ha.ID().Pretty()})
	}
	streamHandler := network.StreamHandler(handleStream)
	if *target == "" {
		log.Println("listening for connections")
		ha.SetStreamHandler("/p2p/1.0.0", streamHandler)
		fmt.Printf("proposenode,%s\n", ha.ID().Pretty())
		select {}

	} else {
		ha.SetStreamHandler("/p2p/1.0.0", streamHandler)
		ipfsaddr, err := multiaddr.NewMultiaddr(*target)
		if err != nil {
			log.Fatalln(err)
		}

		pid, err := ipfsaddr.ValueForProtocol(multiaddr.P_IPFS)
		if err != nil {
			log.Fatalln(err)
		}

		peerid, err := peer.Decode(pid)
		if err != nil {
			log.Fatalln(err)
		}
		targetPeerAddr, _ := multiaddr.NewMultiaddr(
			fmt.Sprintf("/ipfs/%s", peer.Encode(peerid)))
		targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)

		ha.Peerstore().AddAddr(peerid, targetAddr, peerstore.PermanentAddrTTL)

		log.Println("opening stream")

		s, err := ha.NewStream(context.Background(), peerid, "/p2p/1.0.0")
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("Opened new stream to peer:", peerid)

		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

		go writeData(rw)
		go readData(rw)
		fmt.Printf("proposenode,%s\n", ha.ID().Pretty())
		select {} // hang forever

	}
}

func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		log.Println("Error: 1")
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		log.Println("Error: previous block hash => oldBlock.Hash == newBlock.PrevHash")
		return false
	}

	if calculateHash(newBlock) != newBlock.Hash {
		log.Println("Error: calculate hash => calculateHash(newBlock) == newBlock.Hash ")
		return false
	}

	// todo: create add authorizated node function, implement it and implement initialize auth nodes

	if !isNodeAuthorized(newBlock.Validator) {
		log.Println("Error: isNodeAuthorized")

		return false
	}

	return true
}

// cryptographic methods for node authorization are safer

func isNodeAuthorized(nodeID string) bool {
	for _, id := range AuthorizedNodes {
		if id == nodeID {
			return true
		}
	}
	return false
}

// SHA256 hashing
func calculateHash(block Block) string {
	record := (strconv.Itoa(block.Index) +
		block.Timestamp +
		block.Location +
		block.Owner +
		block.Validator +
		block.ProductID +
		block.PrevHash)

	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// create a new block using previous block's hash

func generateBlock(oldBlock Block, productID, location, owner, validator string) Block {
	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.ProductID = productID
	newBlock.Location = location
	newBlock.Owner = owner
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Validator = validator
	newBlock.Hash = calculateHash(newBlock)

	return newBlock
}

// productID,location,owner,validator

// 01, managua, musa,

// 02, managua, musa
