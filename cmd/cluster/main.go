package main

import (
	"time"

	"github.com/EssumanG/distributed-system/internal/node"
)

func main() {
	nodes := map[string]string{
		"node4": "localhost:8001",
		"node2": "localhost:8002",
		"node3": "localhost:8003",
	}

	for id, addr := range nodes {
		peers := map[string]string{}

		for pid, paddr := range nodes {
			if pid != id {
				peers[pid] = paddr
			}
		}
		n := node.NewMode(id, addr, peers)
		go n.Start()

	}
	// Keep cluster running
	for {
		time.Sleep(10 * time.Second)
	}
}
