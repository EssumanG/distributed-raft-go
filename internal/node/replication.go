package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	pb "github.com/EssumanG/distributed-system/internal/rpc"
)

var (
	ErrNotLeader    = errors.New("not the leader")
	ErrTimeout      = errors.New("Operation timeout")
	ErrNotCommitted = errors.New("entry not commited")
)

// ClientCommand represent a command submitted by a client
type ClientCommand struct {
	Data []byte
}
type CommandResult struct {
	Success  bool   //Whether comman was committed
	Term     int32  //Term when the command was stored
	Index    int32  //Index where the command was stored
	LeaderID string // Current leader (for redirect if not leader)
}

// Submit a command to the cluster
// This should only be called on the leader
func (n *Node) Submit(ctx context.Context, command []byte) (*CommandResult, error) {
	n.mu.Lock()

	//Only the leader accept the command
	if n.state != Leader {
		leaderID := n.votedFor
		n.mu.Unlock()

		log.Printf("[Node %s] Rejecting command - not the leader", n.id)
		return &CommandResult{
			Success:  false,
			LeaderID: leaderID,
		}, ErrNotLeader
	}

	//Append to our log
	index := int32(len(n.log)) + 1
	term := n.currentTerm

	entry := LogEntry{
		Term:    term,
		Index:   index,
		Command: command,
	}

	n.log = append(n.log, entry)

	log.Printf("[Node %s] Appended entery at index %d: %s", n.id, index, string(command))

	n.mu.Unlock()

	// replicate log
	go n.replicateLog()

	// Wait to commit ( with timeout)
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//Poll until commited or timeout
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return &CommandResult{
				Success:  false,
				LeaderID: n.id,
				Term:     term,
				Index:    index,
			}, ErrTimeout
		case <-ticker.C:
			n.mu.RLock()
			committed := n.commitIndex >= index
			currentTerm := n.currentTerm
			isLeader := n.state == Leader
			n.mu.RUnlock()

			//Check if we're still leader
			if !isLeader {
				return &CommandResult{
					Success:  false,
					LeaderID: n.id,
					Term:     term,
					Index:    index,
				}, ErrNotLeader
			}

			//Check if term changed (we were deposed)
			if currentTerm != term {
				return &CommandResult{
					Success:  false,
					LeaderID: n.id,
					Term:     term,
					Index:    index,
				}, ErrNotCommitted
			}

			// Check if committed
			if committed {
				log.Printf("[Node %s] Command at index %d committed!", n.id, index)
				return &CommandResult{
					Success:  true,
					LeaderID: n.id,
					Term:     term,
					Index:    index,
				}, ErrTimeout
			}

		}
	}

}

func (n *Node) replicateLog() {
	n.mu.RLock()

	//Check if is leader
	if !n.IsLeader() {
		n.mu.RUnlock()
		return
	}

	currentTerm := n.currentTerm
	leaderID := n.id
	leaderCommit := n.commitIndex

	type replicationInfo struct {
		peerAddr     string
		nextIndex    int32
		prevLogIndex int32
		prevLogTerm  int32
		entries      []LogEntry
	}

	var replicas []replicationInfo
	for _, peerAddr := range n.peers {
		nextIdx := n.nextIndex[peerAddr]
		if nextIdx == 0 {
			nextIdx = 1
		}

		// Calculate prevLog info
		prevLogIndex := nextIdx - 1
		prevLogTerm := int32(0)
		if prevLogIndex > 0 && prevLogIndex <= int32(len(n.log)) {
			prevLogTerm = n.log[prevLogIndex-1].Term
		}

		// Get entries to send
		var entries []LogEntry
		if nextIdx <= int32(len(n.log)) {
			entries = make([]LogEntry, len(n.log[nextIdx-1:]))
			copy(entries, n.log[nextIdx-1:])
		}

		replicas = append(replicas, replicationInfo{
			peerAddr:     peerAddr,
			nextIndex:    nextIdx,
			prevLogIndex: prevLogIndex,
			prevLogTerm:  prevLogTerm,
			entries:      entries,
		})
	}

	n.mu.RUnlock()

	// Send to each peer in parallel
	for _, replica := range replicas {
		go n.sendAppendEntries(replica.peerAddr, currentTerm, leaderID,
			replica.prevLogIndex, replica.prevLogTerm, replica.entries, leaderCommit)
	}
}

// sendAppendEntries sends AppendEntries RPC to a specific peer
func (n *Node) sendAppendEntries(peerAddr string, term int32, leaderID string,
	prevLogIndex int32, prevLogTerm int32, entries []LogEntry, leaderCommit int32) {

	client, ok := n.peerClients[peerAddr]
	if !ok {
		return
	}

	// Convert entries to protobuf format
	var pbEntries []*pb.LogEntry
	for _, entry := range entries {
		pbEntries = append(pbEntries, &pb.LogEntry{
			Term:    entry.Term,
			Index:   entry.Index,
			Command: entry.Command,
		})
	}

	req := &pb.AppendEntriesRequest{
		Term:         term,
		LeaderId:     leaderID,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      pbEntries,
		LeaderCommit: leaderCommit,
	}

	if len(entries) > 0 {
		log.Printf("[Node %s] Sending %d entries to %s (prevLog: idx=%d, term=%d)",
			n.id, len(entries), peerAddr, prevLogIndex, prevLogTerm)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	resp, err := client.AppendEntries(ctx, req)
	if err != nil {
		log.Printf("[Node %s] AppendEntries to %s failed: %v", n.id, peerAddr, err)
		return
	}

	// Handle response
	n.mu.Lock()
	defer n.mu.Unlock()

	// Check if we're still leader in same term
	if n.state != Leader || n.currentTerm != term {
		return
	}

	// If peer has higher term, step down
	if resp.Term > term {
		log.Printf("[Node %s] Peer %s has higher term %d, stepping down", n.id, peerAddr, resp.Term)
		n.currentTerm = resp.Term
		n.state = Follower
		n.votedFor = ""
		return
	}

	if resp.Success {
		// Update nextIndex and matchIndex
		if len(entries) > 0 {
			lastIndex := entries[len(entries)-1].Index
			n.nextIndex[peerAddr] = lastIndex + 1
			n.matchIndex[peerAddr] = lastIndex

			log.Printf("[Node %s] Peer %s accepted entries up to index %d", n.id, peerAddr, lastIndex)

			// Try to update commit index
			n.updateCommitIndex()
		}
	} else {
		// Log inconsistency - decrement nextIndex and retry
		n.nextIndex[peerAddr]--
		if n.nextIndex[peerAddr] < 1 {
			n.nextIndex[peerAddr] = 1
		}

		log.Printf("[Node %s] Peer %s rejected entries, decrementing nextIndex to %d",
			n.id, peerAddr, n.nextIndex[peerAddr])

		// Retry immediately
		go func() {
			n.mu.RLock()
			nextIdx := n.nextIndex[peerAddr]
			prevIdx := nextIdx - 1
			prevTerm := int32(0)
			if prevIdx > 0 && prevIdx <= int32(len(n.log)) {
				prevTerm = n.log[prevIdx-1].Term
			}

			var retryEntries []LogEntry
			if nextIdx <= int32(len(n.log)) {
				retryEntries = make([]LogEntry, len(n.log[nextIdx-1:]))
				copy(retryEntries, n.log[nextIdx-1:])
			}
			n.mu.RUnlock()

			n.sendAppendEntries(peerAddr, term, leaderID, prevIdx, prevTerm, retryEntries, leaderCommit)
		}()
	}
}

// updateCommitIndex updates the commit index if majority of nodes have replicated
func (n *Node) updateCommitIndex() {
	// Find the highest index replicated on majority
	for idx := n.commitIndex + 1; idx <= int32(len(n.log)); idx++ {
		if n.log[idx-1].Term != n.currentTerm {
			// Only commit entries from current term
			continue
		}

		// Count how many nodes have this index
		count := 1 // Count self
		for _, matchIdx := range n.matchIndex {
			if matchIdx >= idx {
				count++
			}
		}

		// Check if majority
		majority := (len(n.peers)+1)/2 + 1
		if count >= majority {
			n.commitIndex = idx
			log.Printf("[Node %s] 🎉 Committed entries up to index %d (replicated on %d/%d nodes)",
				n.id, idx, count, len(n.peers)+1)
		}
	}
}

// GetCommittedEntries returns all committed log entries
func (n *Node) GetCommittedEntries() []LogEntry {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.commitIndex == 0 {
		return nil
	}

	entries := make([]LogEntry, n.commitIndex)
	copy(entries, n.log[:n.commitIndex])
	return entries
}

// GetLogStatus returns current log statistics
func (n *Node) GetLogStatus() string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return fmt.Sprintf("Log: %d entries, Committed: %d, Applied: %d",
		len(n.log), n.commitIndex, n.lastApplied)
}
