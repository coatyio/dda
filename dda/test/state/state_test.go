//go:build testing

// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package state_test provides end-to-end test and benchmark functions for the
// state synchronization service to be tested with different state bindings.
package state_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/services/state/api"
	"github.com/coatyio/dda/testdata"
	"github.com/stretchr/testify/assert"
)

var testServices = map[string]config.ConfigStateService{
	"Raft": {
		Protocol:  "raft",
		Store:     "", // in-memory
		Bootstrap: false,
		Disabled:  false,
		Opts: map[string]any{
			"debug": true, // enable Raft debug logging for task test-log
		},
	},
}

var requiredService = config.ConfigComService{
	Protocol: "mqtt5",
	Url:      "tcp://localhost:1990",
	Auth:     config.AuthOptions{},
	Opts: map[string]any{
		"debug":          true, // enable paho debug logging for task test-log
		"strictClientId": true,
	},
}

var testComSetup = make(testdata.CommunicationSetup)

func init() {
	// Set up required pub-sub communication infrastructure.
	testComSetup["mqtt5"] = &testdata.CommunicationSetupOptions{
		SetupOpts: map[string]any{
			"brokerPort":          1990,
			"brokerWsPort":        0, // WebSocket connection setup not needed
			"brokerLogInfo":       false,
			"brokerLogDebugHooks": false,
		},
	}
}

func TestMain(m *testing.M) {
	testdata.RunWithComSetup(func() { m.Run() }, testComSetup)
}

func TestState(t *testing.T) {
	for name, srv := range testServices {
		t.Run(name, func(t *testing.T) {
			RunTestStateService(t, name, srv, requiredService)
		})
	}
}

func BenchmarkState(b *testing.B) {
	// This benchmark with setup will not be measured itself and called once
	// with b.N=1

	for name, srv := range testServices {
		// This subbenchmark with setup will not be measured itself and called
		// once with b.N=1
		b.Run(name, func(b *testing.B) {
			RunBenchStateService(b, name, srv, requiredService)
		})
	}
}

// RunTestStateService runs all tests on the given state synchronization service
// configuration which requires the given communication service configuration.
func RunTestStateService(t *testing.T, cluster string, srv config.ConfigStateService, req config.ConfigComService) {
	cfg1 := testdata.NewStateConfig(cluster, srv, req)
	cfg1.Services.State.Bootstrap = true // first member creates state synchronization group as leader
	dda1, err := testdata.OpenDdaWithConfig(cfg1)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open bootstrapping dda1")
	}

	var memberChangeCh1 <-chan api.MembershipChange
	memberChangeCtx1, memberChangeCancel1 := context.WithCancel(context.Background())

	t.Run("ObserveMemberChange on leader dda1", func(t *testing.T) {
		ch, err := dda1.ObserveMembershipChange(memberChangeCtx1)
		assert.NotNil(t, ch)
		assert.NoError(t, err)
		memberChangeCh1 = ch
		initial := <-memberChangeCh1
		assert.Equal(t, dda1.Identity().Id, initial.Id)
		assert.True(t, initial.Joined)
	})

	var stateChangeCh1 <-chan api.Input
	stateChangeCtx1, stateChangeCancel1 := context.WithCancel(context.Background())

	t.Run("ObserveStateChange on leader dda1", func(t *testing.T) {
		ch, err := dda1.ObserveStateChange(stateChangeCtx1)
		assert.NotNil(t, ch)
		assert.NoError(t, err)
		stateChangeCh1 = ch
		select {
		case <-time.After(200 * time.Millisecond): // no initial changes
		case sc := <-stateChangeCh1:
			// No synthetic inputs are generated as initial state is empty.
			assert.FailNow(t, "unexpected initial state change", sc)
		}
	})

	t.Run("ProposeInput InputOpSet on leader dda1", func(t *testing.T) {
		val := []byte("foo")
		in := api.Input{Op: api.InputOpSet, Key: "foo", Value: val}
		err := dda1.ProposeInput(context.Background(), &in)
		assert.NoError(t, err)

		sc := <-stateChangeCh1
		assert.Equal(t, in, sc)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case sc := <-stateChangeCh1:
			assert.FailNow(t, "unexpected state change", sc)
		}
	})

	cfg2 := testdata.NewStateConfig(cluster, srv, req)

	// Trigger Raft snapshotting in final test "Trigger snapshotting on
	// dda2/dda3" after every 40 applied log entries checking every 50ms.
	cfg2.Services.State.Opts["snapshotInterval"] = 50
	cfg2.Services.State.Opts["snapshotThreshold"] = 40

	// Shorten response timeout to achieve faster test execution for Propose
	// operation in test "ProposeInput InputOpSet on dda2 after leader
	// election".
	cfg2.Services.State.Opts["lfwTimeout"] = 5000
	dda2, err := testdata.OpenDdaWithConfig(cfg2)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open follower dda2")
	}
	defer testdata.CloseDda(dda2)

	t.Run("Membership change by dda2 on leader dda1", func(t *testing.T) {
		mc := <-memberChangeCh1
		assert.Equal(t, dda2.Identity().Id, mc.Id)
		assert.True(t, mc.Joined)
	})

	var memberChangeCh2 <-chan api.MembershipChange
	memberChangeCtx2, memberChangeCancel2 := context.WithCancel(context.Background())
	defer memberChangeCancel2()

	t.Run("ObserveMemberChange on follower dda2", func(t *testing.T) {
		ch, err := dda2.ObserveMembershipChange(memberChangeCtx2)
		assert.NotNil(t, ch)
		assert.NoError(t, err)
		memberChangeCh2 = ch
		expectedIds := []string{dda1.Identity().Id, dda2.Identity().Id}
		initial1 := <-memberChangeCh2
		assert.Contains(t, expectedIds, initial1.Id)
		assert.True(t, initial1.Joined)
		initial2 := <-memberChangeCh2
		assert.Contains(t, expectedIds, initial2.Id)
		assert.True(t, initial2.Joined)
		assert.NotEqual(t, initial1.Id, initial2.Id)

		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case mc := <-memberChangeCh2:
			assert.FailNow(t, "unexpected membership change", mc)
		}
	})

	var stateChangeCh2 <-chan api.Input
	stateChangeCtx2, stateChangeCancel2 := context.WithCancel(context.Background())
	defer stateChangeCancel2()

	t.Run("ObserveStateChange on follower dda2", func(t *testing.T) {
		ch, err := dda2.ObserveStateChange(stateChangeCtx2)
		assert.NotNil(t, ch)
		assert.NoError(t, err)
		stateChangeCh2 = ch
		select {
		case <-time.After(200 * time.Millisecond):
			assert.FailNow(t, "missing state change")
		case sc := <-stateChangeCh2:
			assert.Equal(t, api.Input{Op: api.InputOpSet, Key: "foo", Value: []byte("foo")}, sc)
		}
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case sc := <-stateChangeCh2:
			assert.FailNow(t, "unexpected state change", sc)
		}
	})

	t.Run("ProposeInput InputOpDelete on follower dda2", func(t *testing.T) {
		in := api.Input{Op: api.InputOpDelete, Key: "foo"}
		err := dda2.ProposeInput(context.Background(), &in)
		assert.NoError(t, err)

		sc := <-stateChangeCh2
		assert.Equal(t, in, sc)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case sc := <-stateChangeCh2:
			assert.FailNow(t, "unexpected state change", sc)
		}

		sc = <-stateChangeCh1
		assert.Equal(t, in, sc)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case sc := <-stateChangeCh1:
			assert.FailNow(t, "unexpected state change", sc)
		}
	})

	cfg3 := testdata.NewStateConfig(cluster, srv, req)
	cfg3.Services.State.Opts["snapshotInterval"] = cfg2.Services.State.Opts["snapshotInterval"]
	cfg3.Services.State.Opts["snapshotThreshold"] = cfg2.Services.State.Opts["snapshotThreshold"]
	dda3, err := testdata.OpenDdaWithConfig(cfg3)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open follower dda3")
	}
	defer testdata.CloseDda(dda3)

	t.Run("Membership change by dda3 on leader dda1", func(t *testing.T) {
		mc := <-memberChangeCh1
		assert.Equal(t, dda3.Identity().Id, mc.Id)
		assert.True(t, mc.Joined)
	})

	t.Run("Membership change by dda3 on follower dda2", func(t *testing.T) {
		mc := <-memberChangeCh2
		assert.Equal(t, dda3.Identity().Id, mc.Id)
		assert.True(t, mc.Joined)
	})

	var memberChangeCh3 <-chan api.MembershipChange
	memberChangeCtx3, memberChangeCancel3 := context.WithCancel(context.Background())
	defer memberChangeCancel3()

	t.Run("ObserveMemberChange on follower dda3", func(t *testing.T) {
		ch, err := dda3.ObserveMembershipChange(memberChangeCtx3)
		assert.NotNil(t, ch)
		assert.NoError(t, err)
		memberChangeCh3 = ch
		expectedIds := []string{dda1.Identity().Id, dda2.Identity().Id, dda3.Identity().Id}
		initial1 := <-memberChangeCh3
		assert.Contains(t, expectedIds, initial1.Id)
		assert.True(t, initial1.Joined)
		initial2 := <-memberChangeCh3
		assert.Contains(t, expectedIds, initial2.Id)
		assert.True(t, initial2.Joined)
		assert.NotEqual(t, initial2.Id, initial1.Id)
		initial3 := <-memberChangeCh3
		assert.Contains(t, expectedIds, initial3.Id)
		assert.True(t, initial3.Joined)
		assert.NotEqual(t, initial3.Id, initial1.Id)
		assert.NotEqual(t, initial3.Id, initial2.Id)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case mc := <-memberChangeCh3:
			assert.FailNow(t, "unexpected membership change", mc)
		}
	})

	var stateChangeCh3 <-chan api.Input
	stateChangeCtx3, stateChangeCancel3 := context.WithCancel(context.Background())
	defer stateChangeCancel3()

	t.Run("ObserveStateChange on follower dda3", func(t *testing.T) {
		ch, err := dda3.ObserveStateChange(stateChangeCtx3)
		assert.NotNil(t, ch)
		assert.NoError(t, err)
		stateChangeCh3 = ch
		select {
		case <-time.After(200 * time.Millisecond): // no changes as state is empty
		case sc := <-stateChangeCh3:
			assert.FailNow(t, "unexpected state change", sc)
		}
	})

	t.Run("ProposeInput InputOpSet on follower dda3", func(t *testing.T) {
		val := []byte("bar")
		in := api.Input{Op: api.InputOpSet, Key: "foo", Value: val}
		err := dda3.ProposeInput(context.Background(), &in)
		assert.NoError(t, err)

		sc := <-stateChangeCh3
		assert.Equal(t, in, sc)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case sc := <-stateChangeCh3:
			assert.FailNow(t, "unexpected state change", sc)
		}

		sc = <-stateChangeCh1
		assert.Equal(t, in, sc)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case sc := <-stateChangeCh1:
			assert.FailNow(t, "unexpected state change", sc)
		}

		sc = <-stateChangeCh2
		assert.Equal(t, in, sc)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case sc := <-stateChangeCh2:
			assert.FailNow(t, "unexpected state change", sc)
		}
	})

	t.Run("Cancel membership change on leader dda1", func(t *testing.T) {
		memberChangeCancel1()
		select {
		case <-time.After(200 * time.Millisecond):
			assert.Fail(t, "timed out without channel closed")
			return
		case mc, ok := <-memberChangeCh1:
			if ok {
				assert.Fail(t, "unexpected membership change", mc)
			}
			return // channel is closed
		}
	})

	t.Run("Cancel state change on leader dda1", func(t *testing.T) {
		stateChangeCancel1()
		select {
		case <-time.After(200 * time.Millisecond):
			assert.Fail(t, "timed out without channel closed")
			return
		case sc, ok := <-stateChangeCh1:
			if ok {
				assert.Fail(t, "unexpected state change", sc)
			}
			return // channel is closed
		}
	})

	t.Run("Late ObserveMemberChange on leader dda1", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch, err := dda1.ObserveMembershipChange(ctx)
		assert.NotNil(t, ch)
		assert.NoError(t, err)
		expectedIds := []string{dda1.Identity().Id, dda2.Identity().Id, dda3.Identity().Id}
		initial1 := <-ch
		assert.Contains(t, expectedIds, initial1.Id)
		assert.True(t, initial1.Joined)
		initial2 := <-ch
		assert.Contains(t, expectedIds, initial2.Id)
		assert.True(t, initial2.Joined)
		assert.NotEqual(t, initial2.Id, initial1.Id)
		initial3 := <-ch
		assert.Contains(t, expectedIds, initial3.Id)
		assert.True(t, initial3.Joined)
		assert.NotEqual(t, initial3.Id, initial1.Id)
		assert.NotEqual(t, initial3.Id, initial2.Id)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case mc := <-ch:
			assert.FailNow(t, "unexpected membership change", mc)
		}

	})

	testdata.CloseDda(dda1) // trigger new leader election between followers dda2 and dda3

	t.Run("Operations on closed leader dda1", func(t *testing.T) {
		in := api.Input{Op: api.InputOpDelete, Key: "foo"}
		err := dda1.ProposeInput(context.Background(), &in)
		assert.Error(t, err)

		ch1, err := dda1.ObserveStateChange(context.Background())
		assert.Nil(t, ch1)
		assert.Error(t, err)

		ch2, err := dda1.ObserveMembershipChange(context.Background())
		assert.Nil(t, ch2)
		assert.Error(t, err)

		assert.NotPanics(t, func() { dda1.Close() }) // already closed
	})

	t.Run("Membership change by dda1 on dda2", func(t *testing.T) {
		mc := <-memberChangeCh2
		assert.Equal(t, dda1.Identity().Id, mc.Id)
		assert.False(t, mc.Joined)
	})

	t.Run("Membership change by dda1 on dda3", func(t *testing.T) {
		mc := <-memberChangeCh3
		assert.Equal(t, dda1.Identity().Id, mc.Id)
		assert.False(t, mc.Joined)
	})

	t.Run("ProposeInput InputOpSet on dda2 after leader election", func(t *testing.T) {
		val := []byte("baz")

		// Note that this ProposeInput operation takes a longer time (configured
		// 5 sec), as leader election is in progress and must be awaited
		// internally using the shortened lfwTimeout for retry with backoff.
		in := api.Input{Op: api.InputOpSet, Key: "foo", Value: val}
		err := dda2.ProposeInput(context.Background(), &in)
		assert.NoError(t, err)

		sc := <-stateChangeCh2
		assert.Equal(t, in, sc)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case sc := <-stateChangeCh2:
			assert.FailNow(t, "unexpected state change", sc)
		}

		sc = <-stateChangeCh3
		assert.Equal(t, in, sc)
		select {
		case <-time.After(200 * time.Millisecond): // no more changes
		case sc := <-stateChangeCh3:
			assert.FailNow(t, "unexpected state change", sc)
		}
	})

	t.Run("Trigger snapshotting on dda2/dda3", func(t *testing.T) {
		// Don't care about hashicorp-raft error logs "[ERROR] raft: failed to
		// take snapshot:". These errors are purely informational and don't
		// affect the proper behavior of the consensus system.
		proposeCount := 100

		go func() {
			i := 0
			keyPrefix := "foo"
			for in := range stateChangeCh2 {
				key := fmt.Sprintf("%s%d", keyPrefix, i)
				assert.Equal(t, api.InputOpSet, in.Op)
				assert.Equal(t, key, in.Key)
				assert.Equal(t, []byte(key), in.Value)
				i++
				if i == proposeCount {
					i = 0
					keyPrefix = "bar"
				}
			}
		}()

		for i := 0; i < proposeCount; i++ {
			key := fmt.Sprintf("foo%d", i)
			in := api.Input{Op: api.InputOpSet, Key: key, Value: []byte(key)}
			err := dda2.ProposeInput(context.Background(), &in)
			assert.NoError(t, err)
		}
		for i := 0; i < proposeCount; i++ {
			key := fmt.Sprintf("bar%d", i)
			in := api.Input{Op: api.InputOpSet, Key: key, Value: []byte(key)}
			err := dda3.ProposeInput(context.Background(), &in)
			assert.NoError(t, err)
		}
	})
}

// RunBenchStateService runs all benchmarks on the given state synchronization
// service.
func RunBenchStateService(b *testing.B, cluster string, srv config.ConfigStateService, req config.ConfigComService) {
	// This benchmark with setup will not be measured itself and called once for
	// each service with b.N=1

	cfg1 := testdata.NewStateConfig(cluster, srv, req)
	cfg1.Services.State.Bootstrap = true // first member creates state synchronization group as leader
	dda1, err := testdata.OpenDdaWithConfig(cfg1)
	if err != nil {
		b.FailNow()
	}
	defer testdata.CloseDda(dda1)

	cfg2 := testdata.NewStateConfig(cluster, srv, req)
	dda2, err := testdata.OpenDdaWithConfig(cfg2)
	if err != nil {
		b.FailNow()
	}
	defer testdata.CloseDda(dda2)

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("ProposeInput by leader", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			key := fmt.Sprintf("leader%d", n)
			err := dda1.ProposeInput(context.Background(), &api.Input{Op: api.InputOpSet, Key: key, Value: []byte(key)})
			if err != nil {
				b.FailNow()
			}
		}
	})

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("ProposeInput by follower", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			key := fmt.Sprintf("follower%d", n)
			err := dda2.ProposeInput(context.Background(), &api.Input{Op: api.InputOpSet, Key: key, Value: []byte(key)})
			if err != nil {
				b.FailNow()
			}
		}
	})
}
