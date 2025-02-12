// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package raft provides a state synchronization binding implementation using
// the [Raft] consensus algorithm.
//
// [Raft]: https://raft.github.io/
package raft

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/plog"
	"github.com/coatyio/dda/services"
	comapi "github.com/coatyio/dda/services/com/api"
	"github.com/coatyio/dda/services/state/api"
	"github.com/hashicorp/go-hclog"
	hraft "github.com/hashicorp/raft"
)

const (
	// DefaultStartupTimeout is the default timeout when starting up a new Raft
	// node, either as a leader or as a follower.
	DefaultStartupTimeout = 10000 * time.Millisecond

	// DefaultLfwTimeout is the default timeout for leaderforwarded Propose and
	// GetState remote operation responses. It only applies in situations where
	// there is no leader. It must not be set too low as Propose operations may
	// take some time.
	DefaultLfwTimeout = 20000 * time.Millisecond
)

// RaftBinding realizes a state synchronization binding for the [Raft] consensus
// protocol by implementing the state synchronization API interface [api.Api]
// using the [HashiCorp Raft] library.
//
// [Raft]: https://raft.github.io/
// [HashiCorp Raft]: https://github.com/hashicorp/raft
type RaftBinding struct {
	raftId     string                      // Raft node ID
	raftDir    string                      // Raft storage location
	mu         sync.Mutex                  // protects following fields
	raft       *hraft.Raft                 // Raft node implementation
	raftLogger hclog.Logger                // Raft logger
	store      *RaftStore                  // Raft LogStore, StableStore, and FileSnapshotStore provider
	fsm        *RaftFsm                    // Raft finite state machine
	trans      *RaftTransport              // Raft transport layer
	lfwTimeout time.Duration               // leader forwarded response timeout
	members    map[hraft.ServerID]struct{} // set of current members
}

// Node returns the Raft node (exposed for testing purposes).
func (b *RaftBinding) Node() *hraft.Raft {
	return b.raft
}

// NodeId returns the Raft node ID (exposed for testing purposes).
func (b *RaftBinding) NodeId() string {
	return b.raftId
}

// Open implements the [api.Api] interface.
func (b *RaftBinding) Open(cfg *config.Config, com comapi.Api) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.raft != nil {
		return nil
	}

	plog.Printf("Open Raft state synchronization binding...\n")

	var err error

	b.raftId = cfg.Identity.Id
	b.members = make(map[hraft.ServerID]struct{})
	b.raftDir = cfg.Services.State.Store

	if b.store, err = NewRaftStore(b.raftDir); err != nil {
		return err
	}

	switch v := cfg.Services.State.Opts["lfwTimeout"].(type) {
	case int:
		b.lfwTimeout = time.Duration(v) * time.Millisecond
	default:
		b.lfwTimeout = DefaultLfwTimeout
	}

	config := hraft.DefaultConfig()
	config.LocalID = hraft.ServerID(b.raftId)

	switch v := cfg.Services.State.Opts["heartbeatTimeout"].(type) {
	case int:
		// Note: heartbeat messages are sent to followers periodically in the
		// range 1x to 2x of config.HeartbeatTimeout / 10.
		config.HeartbeatTimeout = time.Duration(v) * time.Millisecond
	}
	switch v := cfg.Services.State.Opts["electionTimeout"].(type) {
	case int:
		config.ElectionTimeout = time.Duration(v) * time.Millisecond
	}
	switch v := cfg.Services.State.Opts["snapshotInterval"].(type) {
	case int:
		config.SnapshotInterval = time.Duration(v) * time.Millisecond
	}
	switch v := cfg.Services.State.Opts["snapshotThreshold"].(type) {
	case int:
		config.SnapshotThreshold = uint64(v)
	}

	// Increase commit timeout to reduce rate of periodic AppendEntries RPCs to
	// followers (see https://github.com/hashicorp/raft/issues/282).
	config.CommitTimeout = config.HeartbeatTimeout / 10

	config.LogLevel = "OFF" // too many "ERROR" messages are just informational in hashicorp-raft
	if plog.Enabled() {
		config.LogLevel = "ERROR"
		if cfg.Services.State.Opts["debug"] == true {
			config.LogLevel = "DEBUG"
		}
	}
	config.Logger = hclog.New(&hclog.LoggerOptions{
		Name:   "raft",
		Level:  hclog.LevelFromString(config.LogLevel),
		Output: os.Stderr,
	})
	b.raftLogger = config.Logger

	transConfig := &RaftTransportConfig{}
	switch v := cfg.Services.State.Opts["rpcTimeout"].(type) {
	case int:
		transConfig.Timeout = time.Duration(v) * time.Millisecond
	default:
		transConfig.Timeout = 0 // use transport-specific default
	}
	switch v := cfg.Services.State.Opts["installSnapshotTimeoutScale"].(type) {
	case int:
		transConfig.TimeoutScale = v
	default:
		transConfig.TimeoutScale = 0 // use transport-specific default
	}
	b.trans = NewRaftTransport(transConfig, hraft.ServerAddress(b.raftId), com)
	b.fsm = NewRaftFsm()

	// Create Raft node.
	b.raft, err = hraft.NewRaft(config, b.fsm, b.store, b.store, b.store.SnapStore, b.trans)
	if err != nil {
		return err
	}

	startupTimeout := DefaultStartupTimeout
	switch v := cfg.Services.State.Opts["startupTimeout"].(type) {
	case int:
		startupTimeout = time.Duration(v) * time.Millisecond
	}

	if cfg.Services.State.Bootstrap { // bootstrap Raft node as leader
		var configuration hraft.Configuration
		configuration.Servers = append(configuration.Servers, hraft.Server{
			Suffrage: hraft.Voter,
			ID:       hraft.ServerID(b.raftId),
			Address:  hraft.ServerAddress(b.raftId),
		})
		boot := b.raft.BootstrapCluster(configuration)
		if err = boot.Error(); err != nil {
			return err
		}
		select {
		case <-time.After(startupTimeout):
			return fmt.Errorf("timeout on bootstrapping Raft node as a leader")
		case v := <-b.raft.LeaderCh(): // await leadership assignment
			if !v {
				return fmt.Errorf("on bootstrap Raft node should become leader")
			}
		}
	} else { // add Raft node as follower using leader forwarding
		err := services.RetryWithBackoff(context.Background(), services.Backoff{Base: 100 * time.Millisecond, Cap: 100 * time.Millisecond}, func(cnt int) error {
			ctx, cancel := context.WithTimeout(context.Background(), b.lfwTimeout)
			defer cancel()
			if err := b.addVoterByLeaderForwarding(ctx, 0); err == ctx.Err() {
				// If context deadline is exceeded, no response has been sent as
				// there is no leader currently. In this case return a retryable
				// error so that we can reinvoke the operation with a backoff
				// until a new leader is present/elected eventually.
				return services.NewRetryableError(err)
			} else {
				return err
			}
		})
		if err != nil {
			return err
		}
	}

	go b.processLeaderForwardedRPCs()

	return nil
}

// Close implements the [api.Api] interface.
func (b *RaftBinding) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.raft == nil {
		return
	}

	// Turn off Raft logging to silence irrelevant error messages while
	// shuttting down.
	b.raftLogger.SetLevel(hclog.Off)

	if b.raft.State() == hraft.Leader {
		// RemoveServer call finally invokes Shutdown but without closing
		// transport due to a bug in hashicorp-raft: the shutdown future is not
		// awaited so that side effects defined in the future Error function,
		// i.e. closing the transport, are never processed. Even if this bug is
		// fixed some day, the explicit transport Close operation can be kept as
		// all invocations except the first one are no-ops.
		_ = b.raft.RemoveServer(hraft.ServerID(b.raftId), 0, 0).Error()
		_ = b.trans.Close()
	} else {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), b.trans.Timeout())
			defer cancel()
			_ = b.removeServerByLeaderForwarding(ctx, 0)
		}()

		_ = b.raft.Shutdown().Error()
	}

	if err := b.store.Close(false); err != nil { // preserve persistent storage
		plog.Printf("Error closing Raft store: %v\n", err)
	}

	b.raft = nil

	plog.Printf("Closed Raft state synchronization binding\n")
}

// ProposeInput implements the [api.Api] interface.
func (b *RaftBinding) ProposeInput(ctx context.Context, in *api.Input) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.raft == nil {
		return fmt.Errorf("ProposeInput %+v failed as binding is not yet open", in)
	}

	cmd, err := EncodeMsgPack(in)
	if err != nil {
		return err
	}

	// First try to Apply locally, if in follower state forward Apply to leader.
	// Repeat this procedure until a non-error result or a non-retryable error
	// is returned or until the given context is canceled.
	err = services.RetryWithBackoff(ctx, services.Backoff{Base: 100 * time.Millisecond, Cap: 100 * time.Millisecond}, func(cnt int) error {
		f := b.raft.Apply(cmd, 0)
		if f.Error() != nil {
			if f.Error() != hraft.ErrNotLeader {
				return services.NewRetryableError(f.Error())
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), b.lfwTimeout)
				defer cancel()
				if err := b.applyByLeaderForwarding(ctx, cmd, 0); err == ctx.Err() {
					// If context deadline is exceeded, no response has been
					// sent as there is no leader currently. In this case return
					// a retryable error so that we can repropose with a backoff
					// until a new leader is elected eventually.
					return services.NewRetryableError(err)
				} else {
					return err
				}
			}
		} else {
			if err, ok := f.Response().(error); ok {
				return err // non-retryable error by RaftFsm.Apply (never happens in this implementation)
			}
			return nil
		}
	})
	if err != nil {
		return err
	}
	return nil
}

// ObserveStateChange implements the [api.Api] interface.
func (b *RaftBinding) ObserveStateChange(ctx context.Context) (<-chan api.Input, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.raft == nil {
		return nil, fmt.Errorf("ObserveStateChange failed as binding is not yet open")
	}

	stateChangeCh := make(chan api.Input, 256)
	id := b.fsm.AddStateChangeObserver(stateChangeCh)

	go func() {
		defer close(stateChangeCh)
		defer b.fsm.RemoveStateChangeObserver(id)
		<-ctx.Done()
	}()

	return stateChangeCh, nil
}

// ObserveMembershipChange implements the [api.Api] interface.
func (b *RaftBinding) ObserveMembershipChange(ctx context.Context) (<-chan api.MembershipChange, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.raft == nil {
		return nil, fmt.Errorf("ObserveMemberChange failed as binding is not yet open")
	}

	memberChangeCh := make(chan api.MembershipChange, 32)

	// Note that hraft.PeerObservation cannot be used to monitor member changes
	// as they are only observable on the leader node.
	id := b.store.AddMembershipChangeObserver(memberChangeCh, b.raft)

	go func() {
		defer close(memberChangeCh)
		defer b.store.RemoveMembershipChangeObserver(id)
		<-ctx.Done()
	}()

	return memberChangeCh, nil
}

func (b *RaftBinding) addVoterByLeaderForwarding(ctx context.Context, startDelayOnLeader time.Duration) error {
	req := &AddVoterRequest{
		ServerId:      hraft.ServerID(b.raftId),
		ServerAddress: hraft.ServerAddress(b.raftId),
		Timeout:       startDelayOnLeader,
	}
	var resp AddVoterResponse
	if err := b.trans.LfwAddVoter(ctx, req, &resp); err != nil {
		return err
	}
	return nil
}

func (b *RaftBinding) removeServerByLeaderForwarding(ctx context.Context, startDelayOnLeader time.Duration) error {
	req := &RemoveServerRequest{
		ServerId: hraft.ServerID(b.raftId),
		Timeout:  startDelayOnLeader,
	}
	var resp RemoveServerResponse
	if err := b.trans.LfwRemoveServer(ctx, req, &resp); err != nil {
		return err
	}
	return nil
}

func (b *RaftBinding) applyByLeaderForwarding(ctx context.Context, cmd []byte, startDelayOnLeader time.Duration) error {
	req := &ApplyRequest{Command: cmd, Timeout: startDelayOnLeader}
	var resp ApplyResponse
	if err := b.trans.LfwApply(ctx, req, &resp); err != nil {
		return err
	}
	if err := resp.Response; err != nil { // any error response by RaftFsm.Apply is non-retryable
		return err
	}
	return nil
}

func (b *RaftBinding) processLeaderForwardedRPCs() {
	for rpc := range b.trans.LfwConsumer() {
		if b.processLeaderForwardingRPC(rpc) {
			return
		}
	}
}

func (b *RaftBinding) processLeaderForwardingRPC(rpc hraft.RPC) (stop bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.raft == nil {
		stop = true
		rpc.Respond(nil, hraft.ErrNotLeader)
		return
	}

	// All commands fail if the Raft node is not a leader or is transferring
	// leadership. They may also fail on other conditions, such as Raft node
	// shutdown.
	switch cmd := rpc.Command.(type) {
	case *AddVoterRequest:
		if f := b.raft.AddVoter(cmd.ServerId, cmd.ServerAddress, 0, cmd.Timeout); f.Error() != nil {
			rpc.Respond(nil, services.NewRetryableError(f.Error()))
		} else {
			// Wait until all preceding log operations have been applied to leader fsm.
			if fb := b.raft.Barrier(0); fb.Error() != nil {
				rpc.Respond(nil, services.NewRetryableError(fb.Error()))
			} else {
				rpc.Respond(&AddVoterResponse{Index: f.Index()}, nil)
			}
		}
	case *RemoveServerRequest:
		if f := b.raft.RemoveServer(cmd.ServerId, 0, cmd.Timeout); f.Error() != nil {
			rpc.Respond(nil, services.NewRetryableError(f.Error()))
		} else {
			rpc.Respond(&RemoveServerResponse{Index: f.Index()}, nil)
		}
	case *ApplyRequest:
		if f := b.raft.Apply(cmd.Command, cmd.Timeout); f.Error() != nil {
			rpc.Respond(nil, services.NewRetryableError(f.Error()))
		} else {
			if err, ok := f.Response().(error); ok {
				rpc.Respond(&ApplyResponse{Response: err}, nil) // non-retryable error by RaftFsm.Apply
			} else {
				// Wait until all preceding log operations have been applied to leader fsm.
				if fb := b.raft.Barrier(0); fb.Error() != nil {
					rpc.Respond(nil, services.NewRetryableError(fb.Error()))
				} else {
					rpc.Respond(&ApplyResponse{Index: f.Index(), Response: nil}, nil)
				}
			}
		}
	default:
		rpc.Respond(nil, fmt.Errorf("unexpected leader forwarding command %v", cmd))
	}

	return
}
