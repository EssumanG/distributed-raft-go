package node

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

func (n *Node) followLeader() {
	for {
		conn, err := net.Dial("tcp", n.LeaderAddr)
		if err != nil {
			fmt.Printf("[%s] Cannot reach leader\n", n.ID)
			time.Sleep(2 * time.Second)
			continue
		}

		reader := bufio.NewReader(conn)

		for {
			_, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("[%s] Lost leader connection\n", n.ID)
				conn.Close()
				break
			}
			fmt.Printf("[%s] Heartbeat received\n", n.ID)
		}
	}
}
