package main

import (
	"time"

	"github.com/EssumanG/distributed-system/internal/node"
)

func main() {
	nodeIDs := []string{"node1", "node2", "node3"}

	nodes := []*node.Node{}

	for _, id := range nodeIDs {
		peers := []string{}

		for _, peerID := range nodeIDs {
			if peerID != id {
				peers = append(peers, peerID)
			}
		}
		n := node.NewMode(id, peers)
		nodes = append(nodes, n)

	}

	for _, n := range nodes {
		go n.Start()
	}

	// Keep cluster running
	for {
		time.Sleep(5 * time.Second)
	}
}
