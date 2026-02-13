package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/EssumanG/distributed-system/internal/node"
)

func main() {
	nodeID := flag.String("id", "node1", "Unique node identifier")
	port := flag.Int("port", 8001, "Port for this node to listen on")
	peersStr := flag.String("peers", "", "Comma-separated list of peer addresses (e.g., localhose:8002, localhose:8002,)")

	flag.Parse()

	//Parse peers
	var peers []string
	if *peersStr != "" {
		peers = strings.Split(*peersStr, ",")
	}

	// Create node configuration
	cfg := node.Config{
		ID:                *nodeID,
		Peers:             peers,
		ElectionTimeout:   time.Duration(150+randomBetween(0, 150)) * time.Millisecond,
		HeartbeatInterval: 50 * time.Millisecond,
	}

	// Create the node
	n, err := node.NewNode(cfg)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Start the node
	if err := n.Start(ctx); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	log.Printf("===========================================================")
	log.Printf("Node %s started successfully!", *nodeID)
	log.Printf("Listening on port: %d", *port)
	log.Printf("Connected to %d peers: %v", len(peers), peers)
	log.Printf("===========================================================")

	// Print status periodically
	go printStatus(n)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	log.Println("\nReceived shutdown signal...")

	cancel()
	n.Shutdown()

	log.Println("Node stopped gracefully")

}

func printStatus(n *node.Node) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		state, term, id := n.GetState()
		isLeader := n.IsLeader()

		log.Printf("[Node %s] State: %s | Term: %d |Leader: %v", id, state, term, isLeader)
	}
}

func randomBetween(min, max int) int {
	if max <= min {
		return min
	}
	return min + int(time.Now().UnixNano()%(int64(max-min+1)))
}
