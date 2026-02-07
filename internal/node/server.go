package node

import (
	"fmt"
	"net"
	"time"
)

func (n *Node) StartLeaderServer() {
	ln, err := net.Listen("tcp", n.Address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("[%s] Leader Listening on %s\n", n.ID, n.Address)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go n.handleFollower(conn)
	}
}

func (n *Node) handleFollower(conn net.Conn) {
	defer conn.Close()

	for {
		if !n.IsLeader {
			return
		}
		fmt.Fprintln(conn, "HEARTBEAT")
		time.Sleep(2 * time.Second)
	}
}
