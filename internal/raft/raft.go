// internal/raft/raft.go
package raft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// fsm is a no-op implementation of the raft.FSM interface.
type fsm struct{}

func (f *fsm) Apply(*raft.Log) interface{}         { return nil }
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) { return &fsmSnapshot{}, nil }
func (f *fsm) Restore(io.ReadCloser) error         { return nil }

type fsmSnapshot struct{}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error { return sink.Close() }
func (s *fsmSnapshot) Release()                             {}

// Node represents a single node in the Raft cluster.
type Node struct {
	config    *raft.Config
	raft      *raft.Raft
	transport raft.Transport
}

// NewNode creates a new Raft node.
func NewNode(nodeID, raftAddr, raftDir string) (*Node, error) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	// Create the directory for Raft's logs and snapshots
	if err := os.MkdirAll(raftDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create raft directory: %w", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tcp addr: %w", err)
	}
	transport, err := raft.NewTCPTransport(raftAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create tcp transport: %w", err)
	}

	snapshots, err := raft.NewFileSnapshotStore(raftDir, 2, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create bolt store: %w", err)
	}

	ra, err := raft.NewRaft(config, &fsm{}, logStore, logStore, snapshots, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft instance: %w", err)
	}

	return &Node{
		config:    config,
		raft:      ra,
		transport: transport,
	}, nil
}

// BootstrapCluster starts the Raft node and bootstraps the cluster.
func (n *Node) BootstrapCluster() {
	bootstrapConfig := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      n.config.LocalID,
				Address: n.transport.LocalAddr(),
			},
		},
	}
	n.raft.BootstrapCluster(bootstrapConfig)
}

// Join is called by the leader to add a new node to the cluster.
func (n *Node) Join(nodeID, raftAddr string) error {
	if n.raft.State() != raft.Leader {
		return fmt.Errorf("node is not the leader")
	}

	log.Printf("Received join request for node %s at %s", nodeID, raftAddr)
	configFuture := n.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return fmt.Errorf("failed to get raft configuration: %w", err)
	}

	// Check if the node is already a part of the cluster
	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) {
			log.Printf("Node %s already in cluster, skipping join", nodeID)
			return nil
		}
	}

	future := n.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(raftAddr), 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to add voter: %w", err)
	}

	log.Printf("Node %s at %s joined successfully", nodeID, raftAddr)
	return nil
}

// JoinCluster is called by a new node to join an existing cluster.
func (n *Node) JoinCluster(peerHttpAddr string) error {
	joinReq := struct {
		NodeID   string `json:"nodeId"`
		RaftAddr string `json:"raftAddr"`
	}{
		NodeID:   string(n.config.LocalID),
		RaftAddr: string(n.transport.LocalAddr()),
	}

	reqBytes, err := json.Marshal(joinReq)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("%s/join", peerHttpAddr), "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to join cluster, peer returned: %s", resp.Status)
	}

	return nil
}

// LeaderCh returns a channel that notifies on leadership changes.
func (n *Node) LeaderCh() <-chan bool {
	return n.raft.LeaderCh()
}

// IsLeader checks if the current node is the leader.
func (n *Node) IsLeader() bool {
	return n.raft.State() == raft.Leader
}
