package node

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/EssumanG/distributed-system/internal/rpc"
)

type ConsensusClient interface {
	RequestVote(ctx context.Context, req *pb.VoteRequest) (*pb.VoteResponse, error)
	AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error)
	Close() error
}

type grpcConsensusClient struct {
	conn   *grpc.ClientConn
	client pb.ConsensusServiceClient
	addr   string
}

func NewConsensusServiceClient(addr string) (ConsensusClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return nil, fmt.Errorf("Failed to connect to %s: %w", addr, err)
	}

	return &grpcConsensusClient{
		conn:   conn,
		client: pb.NewConsensusServiceClient(conn),
		addr:   addr,
	}, nil
}

func (c *grpcConsensusClient) RequestVote(ctx context.Context, req *pb.VoteRequest) (*pb.VoteResponse, error) {
	return c.client.RequestVote(ctx, req)
}

func (c *grpcConsensusClient) AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	return c.client.AppendEntries(ctx, req)
}

func (c *grpcConsensusClient) Close() error {
	return c.conn.Close()
}

func (n *Node) connectToPeers() error {
	for _, peerAddr := range n.peers {
		client, err := NewConsensusServiceClient(peerAddr)
		if err != nil {
			log.Printf("[Node %s] Failed to connect to peer: %s: %v", n.id, peerAddr, err)
		}

		n.peerClients[peerAddr] = client
		log.Printf("[Node %s]Connected to peer at %s", n.id, peerAddr)
	}

	if len(n.peerClients) == 0 {
		return fmt.Errorf(("Failed to connect to any peers"))
	}

	return nil
}

// requestVote sends a RequestVote RPC to a peer
func (n *Node) requestVote(peerAddr string, term int32) bool {
	client, ok := n.peerClients[peerAddr]
	if !ok {
		log.Printf("[Node %s] No client for peer %s", n.id, peerAddr)
		return false
	}

	n.mu.RLock()
	lastLogIndex := int32(len(n.log))
	lastLogTerm := int32(0)
	if lastLogIndex > 0 {
		lastLogTerm = n.log[lastLogIndex-1].Term
	}
	n.mu.RUnlock()

	req := &pb.VoteRequest{
		Term:         term,
		CandidateId:  n.id,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.RequestVote(ctx, req)
	if err != nil {
		log.Printf("[Node %s] RequestVote to %s failed %v", n.id, peerAddr, err)

		return false
	}

	if resp.Term > term {
		n.stepDown(resp.Term)
		return false
	}
	return resp.VoteGranted
}

func (n *Node) sendHeartbeats() {
	n.mu.RLock()

	if n.state != Leader {
		n.mu.RUnlock()
		return
	}

	currentTerm := n.currentTerm
	leaderID := n.id
	commitIndex := n.commitIndex

	n.mu.RUnlock()

	log.Printf("[Node %s] Sending heartbeats to %d peers (term %d)", n.id, len(n.peers), currentTerm)

	for _, peerAddr := range n.peers {
		go func(addr string) {
			client, ok := n.peerClients[addr]
			if !ok {
				return
			}

			req := &pb.AppendEntriesRequest{
				Term:         currentTerm,
				LeaderId:     leaderID,
				PrevLogIndex: 0,
				PrevLogTerm:  0,
				Entries:      nil,
				LeaderCommit: commitIndex,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

			defer cancel()

			resp, err := client.AppendEntries(ctx, req)

			if err != nil {
				log.Printf("[Node %s] Hearbeat to %s failed: %v", n.id, addr, err)
				return
			}

			//If peer has higer term, step down
			if resp.Term > currentTerm {
				log.Printf("[Node %s] Recieved higher term %d from peer, stepping donw", n.id, resp.Term)
				n.stepDown(resp.Term)
			}

		}(peerAddr)
	}

}
