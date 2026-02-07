package node

import "fmt"

func (n *Node) ElectLeader() {
	leaderID := n.ID
	leaderAddr := n.Address

	for id, addr := range n.Peers {
		if id < leaderID {
			leaderID = id
			leaderAddr = addr
		}
	}

	n.LeaderID = leaderID
	n.LeaderAddr = leaderAddr

	if n.ID == leaderID {
		n.IsLeader = true
		fmt.Printf("[%s] Leader elected\n", n.ID)
		go n.StartLeaderServer()
	} else {
		n.IsLeader = false
		fmt.Printf("[%s] Follower (leader=%s)\n", n.ID, leaderID)
		go n.followLeader()
	}
}
