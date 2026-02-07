package node

import "fmt"

func (n *Node) ElectLeader() {
	leaderID := n.ID

	for _, peer := range n.Peers {
		if peer < leaderID {
			leaderID = peer
		}
	}
	if n.ID == leaderID {
		n.IsLeader = true
		fmt.Printf("[%s] I am the leader!\n", n.ID)
	} else {
		n.IsLeader = false
		fmt.Printf("[%s] I am a follower.\n", n.ID)
	}
}
