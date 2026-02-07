package node

import (
	"fmt"
	"sync"
)

type Node struct {
	ID         string
	Address    string
	IsLeader   bool
	Peers      map[string]string // peerID -> address
	LeaderID   string
	LeaderAddr string
	mu         sync.Mutex
}

func NewMode(id string, addr string, peers map[string]string) *Node {
	return &Node{
		ID:      id,
		Address: addr,
		Peers:   peers,
	}
}

func (n *Node) Start() {
	fmt.Printf("[%s] Node started. Peers %v\n", n.ID, n.Address)

	// go n.StartServer()
	// go n.monitorLeader()

	n.ElectLeader()

	// if n.IsLeader {
	// 	go n.sendHeartbeats()
	// }
}
