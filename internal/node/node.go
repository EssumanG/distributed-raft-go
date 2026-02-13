package node

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// NodeState represents the three possible states in Raft
type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

func (s NodeState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"

	}
}

// Node represents a single node in the distributed system
type Node struct {
	id          string
	currentTerm int32  // Latest term this node has seen
	votedFor    string // CandidateID that recieved in current term
	address     string

	log         []LogEntry
	state       NodeState
	commitIndex int32
	lastApplied int32

	peers       []string // peerID -> address
	peerClients map[string]ConsensusClient

	nextIndex  map[string]int32
	matchIndex map[string]int32

	// Timing and election
	electionTimeout  time.Duration
	heartbeatTimeout time.Duration
	lastHeartbeat    time.Time

	// channel for coordination
	shutdownCh  chan struct{}
	heartbeatCh chan bool
	electionCh  chan bool

	rpcServer *RPCServer

	mu sync.RWMutex // Protects all fields above
}

// LogEntry represents a command in the replicated log
type LogEntry struct {
	Term    int32
	Index   int32
	Command []byte
}

// Config holds node configuration
type Config struct {
	ID                string
	Peers             []string
	ElectionTimeout   time.Duration
	HeartbeatInterval time.Duration
}

func NewNode(cfg Config) (*Node, error) {
	if cfg.ElectionTimeout == 0 {
		// Random timeout between 150-300ms
		cfg.ElectionTimeout = time.Duration(150+rand.Intn(150)) * time.Millisecond
	}
	if cfg.HeartbeatInterval == 0 {
		// Random timeout between 150-300ms
		cfg.HeartbeatInterval = 50 * time.Millisecond
	}

	n := &Node{
		id:               cfg.ID,
		currentTerm:      0,
		state:            Follower,
		peers:            cfg.Peers,
		votedFor:         "",
		commitIndex:      0,
		lastApplied:      0,
		nextIndex:        make(map[string]int32),
		matchIndex:       make(map[string]int32),
		peerClients:      make(map[string]ConsensusClient),
		electionTimeout:  cfg.ElectionTimeout,
		heartbeatTimeout: cfg.HeartbeatInterval,
		lastHeartbeat:    time.Now(),
		shutdownCh:       make(chan struct{}),
		heartbeatCh:      make(chan bool),
		electionCh:       make(chan bool),
	}

	log.Printf("[Node %s] Initialized with election timeout: %v, heartbeat interval: %v",
		n.id, n.electionTimeout, n.heartbeatTimeout)

	return n, nil
}

func (n *Node) Start(ctx context.Context) error {
	log.Printf("[Node %s] starting as %s.", n.id, n.state)

	// Start RPC Server
	if err := n.startRPCServer(); err != nil {
		return fmt.Errorf("Failed to start RPC server: %w", err)
	}

	if err := n.connectToPeers(); err != nil {
		log.Printf("[Node %s] Warning: failed to connect to peers: %v", n.id, err)
	}

	go n.run(ctx)
	return nil
}

func (n *Node) run(ctx context.Context) {
	// Create ticker for election timeout
	electionTicker := time.NewTicker(n.electionTimeout)
	defer electionTicker.Stop()

	// Create ticker for heartbeats (only used when leader)
	heartbeatTicker := time.NewTicker(n.heartbeatTimeout)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Node %s] Shutting down", n.id)
			return

		case <-n.shutdownCh:
			log.Printf("[Node %s] Received shutdown signal", n.id)
			return

		case <-electionTicker.C:
			//Election timeout - start election if we haven't heard from leader
			n.handleElectionTimeout()

		case <-heartbeatTicker.C:
			//Send heartbeats if we're the leader
			n.sendHeartbeats()

		case <-n.heartbeatCh:
			//  Recieved heartbeat from leader - reset election timer
			electionTicker.Reset(n.electionTimeout)

		case <-n.electionCh:
			// State changed - reset timer
			electionTicker.Reset(n.electionTimeout)
		}
	}
}

// GetState return current state and term
func (n *Node) GetState() (NodeState, int32, string) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state, n.currentTerm, n.id
}

func (n *Node) IsLeader() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state == Leader
}

// Shutdown gracefully stopes the node
func (n *Node) Shutdown() {
	log.Printf("[Node %s] Initialting shutdown", n.id)

	close(n.shutdownCh)

	//close peer connections
	for _, client := range n.peerClients {
		if client != nil {
			client.Close()
		}
	}

	//Stop RPC server
	if n.rpcServer != nil {
		n.rpcServer.Stop()
	}

}
