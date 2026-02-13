package node

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	pb "github.com/EssumanG/distributed-system/internal/rpc"
)

// RPCServer handles incoming RPC requests
type RPCServer struct {
	pb.UnimplementedConsensusServiceServer
	node   *Node
	server *grpc.Server
	addr   string
}

func (n *Node) startRPCServer() error {
	// For now, use a simple port based on node ID
	// In production, this would come from config
	port := 8000
	if len(n.id) > 0 && n.id[len(n.id)-1] >= '0' && n.id[len(n.id)-1] <= '9' {
		port = 8000 + int(n.id[len(n.id)-1]-'0')
	}

	addr := fmt.Sprintf(":%d", port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()

	n.rpcServer = &RPCServer{
		node:   n,
		server: grpcServer,
		addr:   addr,
	}

	pb.RegisterConsensusServiceServer(grpcServer, n.rpcServer)

	go func() {
		log.Printf("[Node %s] RPC server listening on %s", n.id, addr)
		if err := grpcServer.Serve(listener); err != nil {
			log.Printf("[Node %s] RPC server error: %v", n.id, err)
		}
	}()

	return nil
}

// func (n *Node) StartLeaderServer() {
// 	ln, err := net.Listen("tcp", n.Address)
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Printf("[%s] Leader Listening on %s\n", n.ID, n.Address)

// 	for {
// 		conn, err := ln.Accept()
// 		if err != nil {
// 			continue
// 		}
// 		go n.handleFollower(conn)
// 	}
// }

// func (n *Node) handleFollower(conn net.Conn) {
// 	defer conn.Close()

// 	for {
// 		if !n.IsLeader {
// 			return
// 		}
// 		fmt.Fprintln(conn, "HEARTBEAT")
// 		time.Sleep(2 * time.Second)
// 	}
// }

func (s *RPCServer) RequestVote(ctx context.Context, req *pb.VoteRequest) (*pb.VoteResponse, error) {
	s.node.mu.Lock()
	defer s.node.mu.Unlock()

	log.Printf("[Node %s] Recieved RequestVote from %s for term %d (our term: %d)",
		s.node.id, req.CandidateId, req.Term, s.node.currentTerm)

	resp := &pb.VoteResponse{
		Term:        s.node.currentTerm,
		VoteGranted: false,
	}

	// 1. Reply false if term < currentTerm
	if req.Term < s.node.currentTerm {
		log.Printf("[Node %s] Rejecting vote - candidate term %d < our term %d", s.node.id, req.Term, s.node.currentTerm)
		return resp, nil
	}

	// 2. If term > currentTerm, update and step down
	if req.Term > s.node.currentTerm {
		log.Printf("[Node %s] updating term from %d to %d and stepping down", s.node.id, s.node.currentTerm, req.Term)
		s.node.currentTerm = req.Term
		s.node.state = Follower
		s.node.votedFor = ""
		s.node.lastHeartbeat = time.Now()
	}

	// 3. Check if we can grant vote
	// Grant vote if:
	// - Ve haven't voted for anyone else in this term
	// - Candidate's log is at least as up-to-date as ours
	canGrant := (s.node.votedFor == "" || s.node.votedFor == req.CandidateId)

	if canGrant {
		// Checkk log up-to-date condition
		lastLogIndex := int32(len(s.node.log))
		lastLogTerm := int32(0)
		if lastLogIndex > 0 {
			lastLogTerm = s.node.log[lastLogIndex-1].Term
		}

		logIsUpToDate := req.LastLogTerm > lastLogTerm || (req.LastLogTerm == lastLogTerm && req.LastLogIndex >= lastLogIndex)

		if logIsUpToDate {
			s.node.votedFor = req.CandidateId
			s.node.lastHeartbeat = time.Now()
			resp.VoteGranted = true
			resp.Term = s.node.currentTerm

			log.Printf("[Node %s] Granted vote to %s for term %d", s.node.id, req.CandidateId, req.Term)

			select {
			case s.node.heartbeatCh <- true:
			default:
			}

		} else {
			log.Printf("[Node %s] Rejecting vote - candidate log not up-to-date", s.node.id)

		}
	} else {
		log.Printf("[Node %s] Rejecting vote- already voted for %s", s.node.id, s.node.votedFor)
	}
	return resp, nil
}

func (s *RPCServer) AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	s.node.mu.Lock()
	defer s.node.mu.Unlock()

	isHearbeat := len(req.Entries) == 0

	if isHearbeat {
		log.Printf("[Node %s] Received heartbeat from leader %s (term %d)", s.node.id, req.LeaderId, req.Term)
	} else {
		log.Printf("[Node %s] Received AppendEntries from %s with %d entries", s.node.id, req.LeaderId, len(req.Entries))
	}

	resp := &pb.AppendEntriesResponse{
		Term:    s.node.currentTerm,
		Success: false,
	}

	// 1. Reply false if term < currentTerm
	if req.Term < s.node.currentTerm {
		log.Printf("[Node %s] Rejecting AppendEntries - leader term %d < our term %d", s.node.id, req.Term, s.node.currentTerm)
		return resp, nil
	}

	// 2. If term > currentTerm, update and convert to follwer
	if req.Term > s.node.currentTerm {
		if s.node.state != Follower {
			log.Printf("[Node %s] Converting to Follower due to AppenEntries from %s", s.node.id, req.LeaderId)
		}
		s.node.currentTerm = req.Term
		s.node.state = Follower
		s.node.votedFor = ""
	}

	//Reset elction timeout
	s.node.lastHeartbeat = time.Now()

	//Signal heartbeat recieved
	select {
	case s.node.heartbeatCh <- true:
	default:
	}

	//For now just accept heartbeats
	//Log replication will be implemented later
	resp.Success = true
	resp.Term = s.node.currentTerm

	return resp, nil

}
func (s *RPCServer) Stop() {
	if s.server != nil {
		log.Printf("[RPC Server] Stopping ...")
		s.server.GracefulStop()
	}
}
