package node

import (
	"fmt"
	"sync"
)

type Node struct {
	ID        string
	Address   string
	Peers     []string
	IsLeader  bool
	Heartbeat chan string
	Log       []string
	mu        sync.Mutex
}

func NewMode(id string, peers []string) *Node {
	return &Node{
		ID:       id,
		Peers:    peers,
		IsLeader: false,
	}
}

func (n *Node) Start() {
	fmt.Printf("[%s] Node started. Peers %v\n", n.ID, n.Peers)
	n.ElectLeader()
}
