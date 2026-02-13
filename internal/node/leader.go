package node

import (
	"log"
	"time"
)

// func (n *Node) ElectLeader() {
// 	leaderID := n.ID
// 	leaderAddr := n.Address

// 	for id, addr := range n.Peers {
// 		if id < leaderID {
// 			leaderID = id
// 			leaderAddr = addr
// 		}
// 	}

// 	n.LeaderID = leaderID
// 	n.LeaderAddr = leaderAddr

// 	if n.ID == leaderID {
// 		n.IsLeader = true
// 		fmt.Printf("[%s] Leader elected\n", n.ID)
// 		go n.StartLeaderServer()
// 	} else {
// 		n.IsLeader = false
// 		fmt.Printf("[%s] Follower (leader=%s)\n", n.ID, leaderID)
// 		go n.followLeader()
// 	}
// }

func (n *Node) becomeLeader(term int32) {
	n.mu.Lock()
	defer n.mu.Unlock()

	//Only become leader if we're still a candidate in the same term
	if n.state != Candidate || n.currentTerm != term {
		return
	}

	log.Printf("[Node %s] WON ELECTION! Becoming leader for term %d", n.id, term)

	n.state = Leader
	go n.sendHeartbeats()
}

// This is called we discover a higher term
func (n *Node) stepDown(newTerm int32) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if newTerm > n.currentTerm {
		log.Printf("[Node %s] Stepping down from %s to Follower (term %d -> %d)", n.id, n.state, n.currentTerm, newTerm)

		n.currentTerm = newTerm
		n.state = Follower
		n.votedFor = ""

		// Reset election timeout
		n.lastHeartbeat = time.Now()

		select {
		case n.electionCh <- true:
		default:
		}
	}
}
