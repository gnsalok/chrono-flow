// internal/raft/raft.go
package raft

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// fsm is a no-op implementation of the raft.FSM interface.
// For our use case, we only care about leadership changes, not replicating data via the FSM.
// The primary state (jobs) is in PostgreSQL, managed by the leader.
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
	transport raft.Transport // We store the transport to access its address later.
}

// NewNode creates a new Raft node.
func NewNode(nodeID, raftAddr, raftDir string) (*Node, error) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	// Create the directory for Raft's logs and snapshots
	if err := os.MkdirAll(raftDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create raft directory: %w", err)
	}

	// Setup the transport layer for Raft nodes to communicate
	addr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tcp addr: %w", err)
	}
	transport, err := raft.NewTCPTransport(raftAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create tcp transport: %w", err)
	}

	// Setup the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(raftDir, 2, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Setup the log store (using BoltDB)
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create bolt store: %w", err)
	}

	// Create the Raft instance
	ra, err := raft.NewRaft(config, &fsm{}, logStore, logStore, snapshots, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft instance: %w", err)
	}

	return &Node{
		config:    config,
		raft:      ra,
		transport: transport, // Store the created transport.
	}, nil
}

// BootstrapCluster starts the Raft node and bootstraps the cluster if it's the first node.
// This should only be done on one node, or if the cluster is being created from scratch.
func (n *Node) BootstrapCluster() {
	bootstrapConfig := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      n.config.LocalID,
				Address: n.transport.LocalAddr(), // Use the stored transport's address.
			},
		},
	}
	n.raft.BootstrapCluster(bootstrapConfig)
}

// JoinCluster attempts to join an existing cluster by contacting a peer.
// NOTE: This is a simplified join mechanism. In a production system, you would typically
// have a separate API endpoint on the leader to handle join requests securely.
func (n *Node) JoinCluster(peerRaftAddr string) error {
	log.Printf("Attempting to join cluster via peer at %s", peerRaftAddr)

	// The AddVoter call must be made to the leader of the cluster.
	// The Raft library will handle forwarding the request if this node is not the leader.
	// However, for a node to even know who the leader is, it must be part of the cluster.
	// This simplified approach requires a more complex join dance or a discovery service.
	// A common pattern is to make an out-of-band API call to a known node,
	// which then as the leader adds the new node as a voter.

	// For the purpose of this project, we assume a simplified model where we just add
	// ourselves as a voter. The Raft library handles the internal RPC to the leader.
	// A more robust implementation would involve a proper discovery and join API.
	if err := n.raft.AddVoter(n.config.LocalID, raft.ServerAddress(peerRaftAddr), 0, 0).Error(); err != nil {
		return fmt.Errorf("failed to add self as voter via peer %s: %w", peerRaftAddr, err)
	}

	return nil
}

// LeaderCh returns a channel that notifies when the node becomes a leader or loses leadership.
func (n *Node) LeaderCh() <-chan bool {
	return n.raft.LeaderCh()
}

// IsLeader checks if the current node is the leader.
func (n *Node) IsLeader() bool {
	return n.raft.State() == raft.Leader
}
