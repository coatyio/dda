// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package raft exports a Raft transport based on DDA pub-sub communication.
package raft

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/coatyio/dda/plog"
	"github.com/coatyio/dda/services"
	comapi "github.com/coatyio/dda/services/com/api"
	hraft "github.com/hashicorp/raft"
)

const (
	// DefaultRpcTimeout is the default remote operation timeout in the Raft
	// transport.
	DefaultRpcTimeout = 1000 * time.Millisecond

	// DefaultInstallSnapshotTimeoutScale is the default TimeoutScale for
	// InstallSnapshot operations in the Raft transport.
	DefaultInstallSnapshotTimeoutScale = 256 * 1024 // 256KB
)

const (
	rpcAppendEntries   string = "ae" // a node-targeted operation
	rpcRequestVote     string = "rv" // a node-targeted operation
	rpcInstallSnapshot string = "is" // a node-targeted operation
	rpcTimeoutNow      string = "tn" // a node-targeted operation
	rpcAddVoter        string = "av" // a leader forwarding operation
	rpcRemoveServer    string = "rs" // a leader forwarding operation
	rpcApply           string = "ap" // a leader forwarding operation
)

const (
	rpcTargetedType        rpcType = "rpc" // action type of node-targeted rpc operations
	rpcLeaderForwardedType rpcType = "lfw" // action type of leader forwarding operations
)

var (
	// ErrTransportShutdown is returned when operations on a transport are
	// invoked after it's been terminated.
	ErrTransportShutdown = fmt.Errorf("transport shutdown")

	// ErrPipelineShutdown is returned when the pipeline is closed.
	ErrPipelineShutdown = fmt.Errorf("append pipeline closed")
)

// rpcType represents specific types of remote operations between Raft nodes.
type rpcType string

// rpcResponse wraps a typed hraft response and a potential error string for a
// retryable or non-retryable error for transmission by pub-sub communication.
type rpcResponse[T any] struct {
	Response  T
	Error     string
	Retryable bool // whether error is retryable
}

// installSnapshotRequestWithData wraps an [hraft.InstallSnapshotRequest] and
// associated snapshot data.
type installSnapshotRequestWithData struct {
	Req  *hraft.InstallSnapshotRequest
	Data []byte // snapshot data
}

// AddVoterRequest represents a leader forwarded request to add this node as a
// voting follower. The request responds with an error if the transport's
// configured rpcTimeout elapses before the corresponding command
// completes on the leader.
type AddVoterRequest struct {
	ServerId      hraft.ServerID
	ServerAddress hraft.ServerAddress
	Timeout       time.Duration // initial time to wait for AddVoter command to be started on leader
}

// AddVoterResponse represents a response to a leader forwarded
// [AddVoterRequest].
type AddVoterResponse struct {
	Index uint64 // holds the index of the newly applied log entry
}

// AddVoterRequest represents a leader forwarded request to remove this node as
// a server from the cluster. The request responds with an error if the
// transport's configured rpcTimeout elapses before the corresponding command
// completes on the leader.
type RemoveServerRequest struct {
	ServerId hraft.ServerID
	Timeout  time.Duration // initial time to wait for RemoveServer command to be started on leader
}

// RemoveServerResponse represents a response to a leader forwarded
// [RemoveServerRequest].
type RemoveServerResponse struct {
	Index uint64 // holds the index of the newly applied log entry
}

// ApplyRequest represents a leader forwarded request to apply a given log
// command. The request responds with an error if the transport's configured
// rpcTimeout elapses before the corresponding command completes on the leader.
type ApplyRequest struct {
	Command []byte
	Timeout time.Duration // initial time to wait for Apply command to be started on leader
}

// ApplyResponse represents a response to a leader forwarded
// [ApplyRequest].
type ApplyResponse struct {
	Index    uint64 // holds the index of the newly applied log entry on the leader
	Response error  // nil if operation is successful; an error otherwise
}

// RaftTransportConfig encapsulates configuration options for the Raft pub-sub
// transport layer.
type RaftTransportConfig struct {
	// Timeout used to apply I/O deadlines to remote operations on the Raft
	// transport. For InstallSnapshot, we multiply the timeout by (SnapshotSize
	// / TimeoutScale).
	//
	// If not present or zero, the default timeout is applied.
	Timeout time.Duration

	// For InstallSnapshot, timeout is proportional to the snapshot size. The
	// timeout is multiplied by (SnapshotSize / TimeoutScale).
	//
	// If not present or zero, a default value of 256KB is used.
	TimeoutScale int
}

// RaftTransport implements the [hraft.Transport] interface to allow Raft to
// communicate with other Raft nodes over the configured DDA pub-sub
// communication protocol. In addition, it supports leader forwarding to allow
// non-leader Raft nodes to accept Apply, Barrier, and AddVoter commands.
type RaftTransport struct {
	localAddr    hraft.ServerAddress
	timeout      time.Duration  // remote operation timeout
	timeoutScale int            // scale factor for InstallSnapshot operation
	consumeCh    chan hraft.RPC // emit node-targeted RPCs
	lfwChan      chan hraft.RPC // emit leader forwarding RPCs

	shutdownCh chan struct{}
	shutdownMu sync.RWMutex // protects shutdown
	shutdown   bool

	heartbeatFnMu sync.Mutex // protects heartbeatFn
	heartbeatFn   func(hraft.RPC)

	com comapi.Api
}

// NewRaftTransport creates a new Raft pub-sub transport with the given
// transport configuration, a local address, and a ready-to-use DDA pub-sub
// communication API.
func NewRaftTransport(config *RaftTransportConfig, addr hraft.ServerAddress, com comapi.Api) *RaftTransport {
	if config == nil {
		config = &RaftTransportConfig{}
	}
	t := &RaftTransport{
		localAddr:    addr,
		timeout:      config.Timeout,
		timeoutScale: config.TimeoutScale,
		consumeCh:    make(chan hraft.RPC),
		lfwChan:      make(chan hraft.RPC),
		shutdownCh:   make(chan struct{}),
		com:          com,
	}
	if t.timeout <= 0 {
		t.timeout = DefaultRpcTimeout
	}
	if t.timeoutScale <= 0 {
		t.timeoutScale = DefaultInstallSnapshotTimeoutScale
	}

	t.subscribeRpcs()

	return t
}

// Timeout gets the configured timeout duration of the transport.
func (t *RaftTransport) Timeout() time.Duration {
	return t.timeout
}

// Consumer returns a channel that can be used to consume and respond to RPC
// requests. This channel is not used for leader forwarding operations (see
// [LfwConsumer]).
//
// Consumer implements the [hraft.Transport] interface.
func (t *RaftTransport) Consumer() <-chan hraft.RPC {
	return t.consumeCh
}

// LocalAddr is used to return our local address to distinguish from our peers.
//
// LocalAddr implements the [hraft.Transport] interface.
func (t *RaftTransport) LocalAddr() hraft.ServerAddress {
	return t.localAddr
}

// AppendEntriesPipeline returns an interface that can be used to pipeline
// AppendEntries requests to the target node.
//
// AppendEntriesPipeline implements the [hraft.Transport] interface.
func (t *RaftTransport) AppendEntriesPipeline(id hraft.ServerID, target hraft.ServerAddress) (hraft.AppendPipeline, error) {
	// Pipelining is not supported by DDA pub-sub transport since no more than
	// one request can be outstanding at once. Skip the whole code path and use
	// synchronous requests.
	return nil, hraft.ErrPipelineReplicationNotSupported
}

// AppendEntries sends the appropriate RPC to the target node.
//
// AppendEntries implements the [hraft.Transport] interface.
func (t *RaftTransport) AppendEntries(id hraft.ServerID, target hraft.ServerAddress, args *hraft.AppendEntriesRequest, resp *hraft.AppendEntriesResponse) error {
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()
	return sendRPC[hraft.AppendEntriesResponse](ctx, t, t.publicationType(rpcTargetedType, target), rpcAppendEntries, args, resp)
}

// RequestVote sends the appropriate RPC to the target node.
//
// RequestVote implements the [hraft.Transport] interface.
func (t *RaftTransport) RequestVote(id hraft.ServerID, target hraft.ServerAddress, args *hraft.RequestVoteRequest, resp *hraft.RequestVoteResponse) error {
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()
	return sendRPC[hraft.RequestVoteResponse](ctx, t, t.publicationType(rpcTargetedType, target), rpcRequestVote, args, resp)
}

// InstallSnapshot is used to push a snapshot down to a follower. The data is
// read from the ReadCloser and streamed to the client.
//
// InstallSnapshot implements the [hraft.Transport] interface.
func (t *RaftTransport) InstallSnapshot(id hraft.ServerID, target hraft.ServerAddress, args *hraft.InstallSnapshotRequest, resp *hraft.InstallSnapshotResponse, data io.Reader) error {
	timeout := t.timeout
	timeout = timeout * time.Duration(args.Size/int64(t.timeoutScale))
	if timeout < t.timeout {
		timeout = t.timeout
	}

	// Send the RPC along with snapshot data.
	//
	// TODO Since DDA pub-sub payload size is limited, we should chunk large
	// snapshot data (using bufio) in multiple dedicated Event messages with a
	// unique Event Type which are dynamically subscribed and decoded by
	// [handleRpc].
	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	req := &installSnapshotRequestWithData{
		Req:  args,
		Data: b,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return sendRPC[hraft.InstallSnapshotResponse](ctx, t, t.publicationType(rpcTargetedType, target), rpcInstallSnapshot, req, resp)
}

// EncodePeer is used to serialize a peer's address
//
// EncodePeer implements the [hraft.Transport] interface.
func (t *RaftTransport) EncodePeer(id hraft.ServerID, addr hraft.ServerAddress) []byte {
	return []byte(addr)
}

// DecodePeer is used to deserialize a peer's address.
//
// DecodePeer implements the [hraft.Transport] interface.
func (t *RaftTransport) DecodePeer(buf []byte) hraft.ServerAddress {
	return hraft.ServerAddress(buf)
}

// SetHeartbeatHandler is used to setup a heartbeat handler as a fast-path. This
// is to avoid head-of-line blocking from RPC invocations. If a Transport does
// not support this, it can simply ignore the callback, and push the heartbeat
// onto the Consumer channel. Otherwise, it MUST be safe for this callback to be
// invoked concurrently with a blocking RPC.
//
// SetHeartbeatHandler implements the [hraft.Transport] interface.
func (t *RaftTransport) SetHeartbeatHandler(cb func(rpc hraft.RPC)) {
	t.heartbeatFnMu.Lock()
	defer t.heartbeatFnMu.Unlock()
	t.heartbeatFn = cb
}

// TimeoutNow is used to start a leadership transfer to the target node.
//
// TimeoutNow implements the [hraft.Transport] interface.
func (t *RaftTransport) TimeoutNow(id hraft.ServerID, target hraft.ServerAddress, args *hraft.TimeoutNowRequest, resp *hraft.TimeoutNowResponse) error {
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()
	return sendRPC[hraft.TimeoutNowResponse](ctx, t, t.publicationType(rpcTargetedType, target), rpcTimeoutNow, args, resp)
}

// Close permanently closes a transport, stopping any associated goroutines and
// freeing other resources.
//
// Close implements the [hraft.WithClose] interface. WithClose is an interface
// that a transport may provide which allows a transport to be shut down cleanly
// when a Raft instance shuts down.
func (t *RaftTransport) Close() error {
	t.shutdownMu.Lock()
	defer t.shutdownMu.Unlock()

	if !t.shutdown {
		close(t.consumeCh)
		t.consumeCh = nil
		close(t.lfwChan)
		t.lfwChan = nil
		close(t.shutdownCh)
		t.shutdown = true
	}
	return nil
}

// LfwConsumer returns a channel that can be used to consume and respond to
// leader forwarded RPC requests. This channel is not used for node-targeted RPC
// operations (see [Consumer]).
func (t *RaftTransport) LfwConsumer() <-chan hraft.RPC {
	return t.lfwChan
}

// LfwAddVoter implements transparent leader forwarding for [hraft.AddVoter]
// command. It will forward the request to the leader which will add the given
// server to the cluster as a staging server, promoting it to a voter once that
// server is ready.
//
// LfwAddVoter is a blocking operation that will time out with an error in case
// no response is received within a time interval given by the context.
func (t *RaftTransport) LfwAddVoter(ctx context.Context, args *AddVoterRequest, resp *AddVoterResponse) error {
	return sendRPC[AddVoterResponse](ctx, t, t.publicationType(rpcLeaderForwardedType), rpcAddVoter, args, resp)
}

// LfwRemoveServer implements transparent leader forwarding for
// [hraft.RemoveServer] command. It will forward the request to the leader which
// will remove the given server from the cluster. If the current leader is being
// removed, it will cause a new election to occur.
//
// LfwRemoveServer is a blocking operation that will time out with an error in
// case no response is received within a time interval given by the context.
func (t *RaftTransport) LfwRemoveServer(ctx context.Context, args *RemoveServerRequest, resp *RemoveServerResponse) error {
	return sendRPC[RemoveServerResponse](ctx, t, t.publicationType(rpcLeaderForwardedType), rpcRemoveServer, args, resp)
}

// LfwApply implements transparent leader forwarding for [hraft.Apply] command.
// It will forward a command to the leader which applies it to the FSM in a
// highly consistent manner.
//
// LfwApply is a blocking operation that will time out with an error in case no
// response is received within a time interval given by the context.
func (t *RaftTransport) LfwApply(ctx context.Context, args *ApplyRequest, resp *ApplyResponse) error {
	return sendRPC[ApplyResponse](ctx, t, t.publicationType(rpcLeaderForwardedType), rpcApply, args, resp)
}

func sendRPC[T any](ctx context.Context, t *RaftTransport, pubType string, rpcOp string, args any, resp *T) error {
	b, err := EncodeMsgPack(args)
	if err != nil {
		return err
	}

	res, err := t.com.PublishAction(ctx, comapi.Action{
		Type:   pubType,
		Id:     rpcOp,
		Source: string(t.localAddr),
		Params: b,
	}, comapi.ScopeState)
	if err != nil {
		return err
	}

	select {
	case <-t.shutdownCh:
		return ErrTransportShutdown
	case ar, ok := <-res:
		if !ok {
			return ctx.Err()
		}
		var res rpcResponse[T]
		if err := DecodeMsgPack(ar.Data, &res); err != nil {
			return err
		}
		if res.Error == "" {
			*resp = res.Response
			return nil
		} else {
			if res.Retryable {
				return services.RetryableErrorf(res.Error)
			} else {
				return fmt.Errorf(res.Error)
			}
		}
	}
}

func sendRPCResponse[T any](t *RaftTransport, resp hraft.RPCResponse, ac comapi.ActionWithCallback) error {
	res := &rpcResponse[*T]{
		Response:  nil,
		Error:     "",
		Retryable: false,
	}
	if resp.Error != nil {
		res.Error = resp.Error.Error()
		res.Retryable = services.IsRetryable(resp.Error)
	} else {
		res.Response = resp.Response.(*T)
	}
	if b, err := EncodeMsgPack(res); err != nil {
		return err
	} else {
		if err := ac.Callback(comapi.ActionResult{
			Context: string(t.localAddr),
			Data:    b,
		}); err != nil {
			return err
		}
		return nil
	}
}

func (t *RaftTransport) publicationType(rpcType rpcType, target ...hraft.ServerAddress) string {
	switch rpcType {
	case rpcTargetedType:
		return fmt.Sprintf("%s%s", rpcTargetedType, target[0])
	default:
		return string(rpcLeaderForwardedType)
	}
}

func (t *RaftTransport) subscriptionType(rpcType rpcType) string {
	switch rpcType {
	case rpcTargetedType:
		return fmt.Sprintf("%s%s", rpcTargetedType, t.localAddr)
	default:
		return string(rpcLeaderForwardedType)
	}
}

func (t *RaftTransport) subscribeRpcs() {
	ctx, cancel := context.WithCancel(context.Background())

	// Subscribe RPCs targeted at this individual Raft node.
	filter := comapi.SubscriptionFilter{
		Scope: comapi.ScopeState,
		Type:  t.subscriptionType(rpcTargetedType),
	}
	acsNode, err := t.com.SubscribeAction(ctx, filter)
	if err != nil {
		plog.Printf("subscribing action type failed: %v", err)
	}

	// Subscribe leader forwarding RPCs targeted at all Raft nodes.
	filter = comapi.SubscriptionFilter{
		Scope: comapi.ScopeState,
		Type:  t.subscriptionType(rpcLeaderForwardedType),
	}
	acsLfw, err := t.com.SubscribeAction(ctx, filter)
	if err != nil {
		plog.Printf("subscribing action type failed: %v", err)
	}

	go func() {
		defer cancel() // Unsubscribe RPC subscriptions on shutdown
		for {
			select {
			case <-t.shutdownCh:
				return // Stop fast on closing transport
			default:
			}

			select {
			case <-t.shutdownCh:
				return
			case ac, ok := <-acsNode:
				if !ok {
					return // stop fast on closing transport
				}
				if err := t.handleRpc(ac); err != nil {
					plog.Printf("failed to handle incoming action %+v: %v", ac.Action, err)
				}
			case ac, ok := <-acsLfw:
				if !ok {
					return // stop fast on closing transport
				}
				if err := t.handleRpc(ac); err != nil {
					plog.Printf("failed to handle incoming action %+v: %v", ac.Action, err)
				}
			}
		}
	}()
}

// handleRpc decodes and dispatches any incoming remote operation. Returns nil
// if a response (including a potential error) could be transmitted
// successfully; otherwise an error is returned.
func (t *RaftTransport) handleRpc(ac comapi.ActionWithCallback) error {
	// Ignore leader forwarded requests that originate from the same Raft node.
	// Such operations must be tried locally first (with success on a leader
	// node) before forwarding them to all other nodes.
	if ac.Type == string(rpcLeaderForwardedType) && ac.Source == string(t.localAddr) {
		return nil
	}

	t.shutdownMu.Lock()
	defer t.shutdownMu.Unlock()

	if t.shutdown {
		// Stop handling action immediately when shutdown is ongoing.
		return nil
	}

	// Create the RPC object
	respCh := make(chan hraft.RPCResponse, 1)
	rpc := hraft.RPC{
		RespChan: respCh,
	}

	var consumeChan chan hraft.RPC

	// Decode the RPC
	isHeartbeat := false
	switch ac.Type {
	case string(rpcLeaderForwardedType): // leader forwarding operations
		consumeChan = t.lfwChan
		switch ac.Id {
		case rpcAddVoter:
			var req AddVoterRequest
			if err := DecodeMsgPack(ac.Params, &req); err != nil {
				return sendRPCResponse[AddVoterResponse](t, hraft.RPCResponse{Error: err}, ac)
			}
			rpc.Command = &req
		case rpcRemoveServer:
			var req RemoveServerRequest
			if err := DecodeMsgPack(ac.Params, &req); err != nil {
				return sendRPCResponse[RemoveServerResponse](t, hraft.RPCResponse{Error: err}, ac)
			}
			rpc.Command = &req
		case rpcApply:
			var req ApplyRequest
			if err := DecodeMsgPack(ac.Params, &req); err != nil {
				return sendRPCResponse[ApplyResponse](t, hraft.RPCResponse{Error: err}, ac)
			}
			rpc.Command = &req
		default:
			return fmt.Errorf("unknown leader forwarding rpc type %s from %s", ac.Id, ac.Source)
		}
	default: // node-targeted operations
		consumeChan = t.consumeCh
		switch ac.Id {
		case rpcAppendEntries:
			var req hraft.AppendEntriesRequest
			if err := DecodeMsgPack(ac.Params, &req); err != nil {
				return sendRPCResponse[hraft.AppendEntriesResponse](t, hraft.RPCResponse{Error: err}, ac)
			}
			rpc.Command = &req

			leaderAddr := req.RPCHeader.Addr

			// Check if this is a heartbeat (sent as AppendEntries request)
			if req.Term != 0 && leaderAddr != nil &&
				req.PrevLogEntry == 0 && req.PrevLogTerm == 0 &&
				len(req.Entries) == 0 && req.LeaderCommitIndex == 0 {
				isHeartbeat = true
			}
		case rpcRequestVote:
			var req hraft.RequestVoteRequest
			if err := DecodeMsgPack(ac.Params, &req); err != nil {
				return sendRPCResponse[hraft.RequestVoteResponse](t, hraft.RPCResponse{Error: err}, ac)
			}
			rpc.Command = &req
		case rpcInstallSnapshot:
			var req installSnapshotRequestWithData
			if err := DecodeMsgPack(ac.Params, &req); err != nil {
				return sendRPCResponse[hraft.InstallSnapshotResponse](t, hraft.RPCResponse{Error: err}, ac)
			}
			rpc.Command = req.Req
			rpc.Reader = bytes.NewReader(req.Data)
		case rpcTimeoutNow:
			var req hraft.TimeoutNowRequest
			if err := DecodeMsgPack(ac.Params, &req); err != nil {
				return sendRPCResponse[hraft.TimeoutNowResponse](t, hraft.RPCResponse{Error: err}, ac)
			}
			rpc.Command = &req
		default:
			return fmt.Errorf("unknown rpc type %s from %s", ac.Id, ac.Source)
		}
	}

	// Check for heartbeat fast-path
	if isHeartbeat {
		t.heartbeatFnMu.Lock()
		fn := t.heartbeatFn
		t.heartbeatFnMu.Unlock()
		if fn != nil {
			fn(rpc)
			goto RESP
		}
	}

	consumeChan <- rpc // dispatch non-heartbeat RPC

RESP:
	resp := <-respCh
	switch ac.Type {
	case string(rpcLeaderForwardedType):
		switch ac.Id {
		case rpcAddVoter:
			return sendLfwResponse[AddVoterResponse](t, resp, ac)
		case rpcRemoveServer:
			return sendLfwResponse[RemoveServerResponse](t, resp, ac)
		case rpcApply:
			return sendLfwResponse[ApplyResponse](t, resp, ac)
		default:
			return fmt.Errorf("unknown leader forwarding rpc type %s from %s", ac.Id, ac.Source)
		}

	default:
		switch ac.Id {
		case rpcAppendEntries:
			return sendRPCResponse[hraft.AppendEntriesResponse](t, resp, ac)
		case rpcRequestVote:
			return sendRPCResponse[hraft.RequestVoteResponse](t, resp, ac)
		case rpcInstallSnapshot:
			return sendRPCResponse[hraft.InstallSnapshotResponse](t, resp, ac)
		case rpcTimeoutNow:
			return sendRPCResponse[hraft.TimeoutNowResponse](t, resp, ac)
		default:
			return fmt.Errorf("unknown rpc type %s from %s", ac.Id, ac.Source)
		}
	}
}

func sendLfwResponse[T any](t *RaftTransport, resp hraft.RPCResponse, ac comapi.ActionWithCallback) error {
	rError := services.ErrorRetryable(resp.Error)
	if rError == nil {
		return sendRPCResponse[T](t, resp, ac)
	}
	// Only respond on errors that are safely originating from the leader.
	switch rError {
	case hraft.ErrNotLeader:
		// Don't respond if we are not leader. This may cause the associated request to
		// time out on the invoker if there is currently no other leader.
		return nil
	case hraft.ErrLeadershipTransferInProgress:
		return sendRPCResponse[T](t, resp, ac) // respond if leader is rejecting client request because of leadership transfer
	case hraft.ErrLeadershipLost:
		return sendRPCResponse[T](t, resp, ac) // respond if leadership is lost while executing command
	case hraft.ErrLeader:
		return sendRPCResponse[T](t, resp, ac) // respond if command can't be completed by leader
	case hraft.ErrAbortedByRestore:
		return sendRPCResponse[T](t, resp, ac) // respond if leader fails to commit log due to snapshot restore
	default:
		// Don't respond on error for which we cannot safely detect if it is
		// originating from a leader. This may cause the associated request to
		// time out on the invoker. Note that this error is logged by subscribeRpcs.
		return rError
	}
}
