package node

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// handleElectionTimeout is called when election timeout fires
func (n *Node) handleElectionTimeout() {
	n.mu.Lock()

	//Only follwers and candidates start elections
	if n.state == Leader {
		n.mu.Unlock()
		return
	}

	// Check if we've received a heartbeat recently
	timeSinceHeartbeat := time.Since(n.lastHeartbeat)
	if timeSinceHeartbeat < n.electionTimeout {
		n.mu.Unlock()
		return
	}

	log.Printf("[Node %s] Election timeout! Time since last heartbeat: %v. Starting election...", n.id, timeSinceHeartbeat)
	n.mu.Unlock()

	n.startElection()

}

func (n *Node) startElection() {
	n.mu.Lock()

	n.currentTerm++
	n.state = Candidate
	n.votedFor = n.id
	currentTerm := n.currentTerm

	log.Printf("[Node %s] Starting election %d", n.id, currentTerm)

	n.mu.Unlock()

	// Reset election timeout
	select {
	case n.electionCh <- true:
	default:
	}

	// Request votes from all peers
	votes := 1
	votesNeeded := (len(n.peers)+1)/2 + 1 //Majority

	var wg sync.WaitGroup
	voteCh := make(chan bool, len(n.peers))
	fmt.Printf("kjjjjjjjjjjjjjjjjjjjjjjii%d", len(n.peers))

	for _, peer := range n.peers {
		wg.Add(1)
		go func(peerAddr string) {
			fmt.Print("kokkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkko")
			defer wg.Done()
			granted := n.requestVote(peerAddr, currentTerm)
			voteCh <- granted
		}(peer)
	}

	go func() {
		wg.Wait()
		close(voteCh)
	}()

	for granted := range voteCh {
		if granted {
			votes++
			log.Printf("[Node %s] Received vote. Total votes: %d/%d", n.id, votes, votesNeeded)

			if votes >= votesNeeded {
				n.becomeLeader(currentTerm)
				return
			}
		}
	}

	log.Printf("[Node %s] Election failed. Only received %d/%d votes", n.id, votes, votesNeeded)

}
