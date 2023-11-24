//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package com provides end-to-end test and benchmark functions for the
// communication service to be tested with different authentication methods over
// different communication bindings.
package com

import (
	"context"
	"testing"
	"time"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda"
	"github.com/coatyio/dda/services"
	"github.com/coatyio/dda/services/com/api"
	"github.com/coatyio/dda/testdata"
	"github.com/stretchr/testify/assert"
)

// Runs all tests on the given communication service for the given pub-sub
// communication setup.
func RunTestComService(t *testing.T, cluster string, comSrv config.ConfigComService, testPubSubSetup testdata.PubSubCommunicationSetup) {
	cfg1 := testdata.NewConfig(cluster, "dda1", comSrv)

	pub, err := testdata.OpenDda(cluster, "pub", comSrv)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA")
	}
	sub1, err := testdata.OpenDda(cluster, "sub1", comSrv)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA")
	}
	sub2, err := testdata.OpenDda(cluster, "sub2", comSrv)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA")
	}

	defer testdata.CloseDda(sub1)
	defer testdata.CloseDda(sub2)
	defer testdata.CloseDda(pub)

	pubDataPrefix := "Hello from pub "
	pubData := []byte(pubDataPrefix + pub.Identity().Id)

	t.Run("Incompatible Config version", func(t *testing.T) {
		cfg := *cfg1
		cfg.Version = "xxx"
		dda, err := dda.New(&cfg)
		assert.Nil(t, dda)
		assert.Error(t, err)
	})
	t.Run("Invalid Config Cluster", func(t *testing.T) {
		cfg := *cfg1
		cfg.Cluster = "foo/bar"
		dda, err := dda.New(&cfg)
		assert.Nil(t, dda)
		assert.Error(t, err)
	})
	t.Run("Unsupported protocol", func(t *testing.T) {
		cfg := *cfg1
		cfg.Services.Com.Protocol = "xxxx"
		dda, err := dda.New(&cfg)
		assert.Nil(t, dda)
		assert.Error(t, err)
	})
	t.Run("Close on unopened", func(t *testing.T) {
		dda, err := dda.New(cfg1)
		assert.NotNil(t, dda)
		assert.NoError(t, err)
		assert.Equal(t, struct{}{}, <-dda.ComApi().Close())
	})
	t.Run("Open with unsupported auth method", func(t *testing.T) {
		cfg := *cfg1
		cfg.Services.Com.Auth.Method = "foo"
		dda, err := dda.New(&cfg)
		assert.NotNil(t, dda)
		assert.NoError(t, err)
		err = dda.Open(500 * time.Millisecond)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))
	})
	t.Run("Open with invalid auth certificate/key", func(t *testing.T) {
		cfg := *cfg1
		cfg.Services.Com.Auth.Method = "tls"
		cfg.Services.Com.Auth.Cert = "foo.pem"
		cfg.Services.Com.Auth.Key = "bar.pem"
		dda, err := dda.New(&cfg)
		assert.NotNil(t, dda)
		assert.NoError(t, err)
		err = dda.Open(500 * time.Millisecond)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))
	})
	t.Run("Open with invalid url schema", func(t *testing.T) {
		cfg := *cfg1
		cfg.Services.Com.Url = "foo\u007Fbar"
		dda, err := dda.New(&cfg)
		assert.NotNil(t, dda)
		assert.NoError(t, err)
		err = dda.Open(500 * time.Millisecond)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))
	})
	t.Run("Open on connection refused", func(t *testing.T) {
		cfg := *cfg1
		cfg.Services.Com.Url += "0"
		dda, err := dda.New(&cfg)
		assert.NotNil(t, dda)
		assert.NoError(t, err)
		err = dda.Open(1 * time.Second)
		assert.Error(t, err)
		assert.True(t, services.IsRetryable(err))
	})

	var dda1 *dda.Dda

	t.Run("Open on closed", func(t *testing.T) {
		var err error
		dda1, err = dda.New(cfg1)
		assert.NotNil(t, dda1)
		assert.NoError(t, err)
		err = dda1.Open(1 * time.Second)
		assert.NoError(t, err)
	})
	t.Run("Open on opened", func(t *testing.T) {
		err := <-dda1.ComApi().Open(cfg1, 0)
		assert.NoError(t, err, "binding already open")
	})
	t.Run("Close on opened", func(t *testing.T) {
		assert.Equal(t, struct{}{}, <-dda1.ComApi().Close())
	})
	t.Run("Close on closed", func(t *testing.T) {
		assert.Equal(t, struct{}{}, <-dda1.ComApi().Close())
	})

	evt := api.Event{
		Type:            "hello",
		Id:              "42",
		Source:          "pub",
		Data:            pubData,
		DataContentType: "hellopub",
	}

	t.Run("PublishEvent with invalid Type", func(t *testing.T) {
		evtType := evt.Type
		evt.Type = "+-*"
		defer func() { evt.Type = evtType }()
		err := pub.PublishEvent(evt)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Event Type "+-*": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("PublishEvent with empty Type", func(t *testing.T) {
		evtType := evt.Type
		evt.Type = ""
		defer func() { evt.Type = evtType }()
		err := pub.PublishEvent(evt)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Event Type "": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("PublishEvent with empty Id", func(t *testing.T) {
		evtId := evt.Id
		evt.Id = ""
		defer func() { evt.Id = evtId }()
		err := pub.PublishEvent(evt)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Event Id "": must not be empty`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("PublishEvent with empty Source", func(t *testing.T) {
		evtSrc := evt.Source
		evt.Source = ""
		defer func() { evt.Source = evtSrc }()
		err := pub.PublishEvent(evt)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Event Source "": must not be empty`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("SubscribeEvent with invalid Filter", func(t *testing.T) {
		events, err := sub1.SubscribeEvent(context.Background(), api.SubscriptionFilter{Type: "foo/bar"})
		assert.Nil(t, events)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Event Type "foo/bar": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("Publish-SubscribeEvent", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		events1, err1 := sub1.SubscribeEvent(ctx1, api.SubscriptionFilter{Type: evt.Type})
		assert.NotNil(t, events1)
		assert.NoError(t, err1)

		ctx2, cancel2 := context.WithCancel(context.Background())
		events2, err2 := sub2.SubscribeEvent(ctx2, api.SubscriptionFilter{Type: evt.Type})
		assert.NotNil(t, events2)
		assert.NoError(t, err2)

		err := pub.PublishEvent(evt)
		assert.NoError(t, err)

		assert.Equal(t, evt, <-events1)
		assert.Equal(t, evt, <-events2)

		cancel1()                               // unsubscribe is asyn, await close before publishing
		assert.Equal(t, api.Event{}, <-events1) // closed by cancel1, emits zero value

		evt.Id = "43"
		err = pub.PublishEvent(evt)
		assert.NoError(t, err)
		assert.Equal(t, evt, <-events2)

		cancel2()                               // unsubscribe is asyn, await close before publishing
		assert.Equal(t, api.Event{}, <-events2) // closed by cancel2, emits zero value

		evt.Id = "44"
		err = pub.PublishEvent(evt, api.ScopeCom)
		assert.NoError(t, err)
		assert.Equal(t, api.Event{}, <-events1) // closed by cancel1, emits zero value
		assert.Equal(t, api.Event{}, <-events2) // closed by cancel2, emits zero value
	})
	t.Run("ResubscribeEvent on reconnect", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		events, err := sub1.SubscribeEvent(ctx, api.SubscriptionFilter{Type: evt.Type})
		assert.NotNil(t, events)
		assert.NoError(t, err)

		// Let pub-sub broker/infastructure gracefully close the connection to
		// the communication binding so that it attempts to reconnect and to
		// resubscribe all active subscriptions.
		//
		// The delay is a bit sensitive: The duration it takes for the
		// communication binding to handle a connection close i.e. trying to
		// reconnect and resubscribe, may take longer than the give timespan
		// causing the events channel to block indefinitely in the receive
		// operation below which prevents the test from terminating.
		testPubSubSetup[comSrv.Protocol].DisconnectBindingFunc(sub1.ComApi(), 2*time.Second)

		err = pub.PublishEvent(evt)
		assert.NoError(t, err)
		assert.Equal(t, evt, <-events)
	})

	act := api.Action{
		Type:   "oneof",
		Id:     "42",
		Source: "pub",
		Params: []byte{1, 2, 3, 4, 5},
	}

	t.Run("PublishAction with invalid Type", func(t *testing.T) {
		actType := act.Type
		act.Type = "+-*"
		defer func() { act.Type = actType }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		res, err := pub.PublishAction(ctx, act)
		assert.Nil(t, res)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Action Type "+-*": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("PublishAction with empty Type", func(t *testing.T) {
		actType := act.Type
		act.Type = ""
		defer func() { act.Type = actType }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		res, err := pub.PublishAction(ctx, act)
		assert.Nil(t, res)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Action Type "": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("PublishAction with empty Id", func(t *testing.T) {
		actId := act.Id
		act.Id = ""
		defer func() { act.Id = actId }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		res, err := pub.PublishAction(ctx, act)
		assert.Nil(t, res)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Action Id "": must not be empty`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("PublishAction with empty Source", func(t *testing.T) {
		actSrc := act.Source
		act.Source = ""
		defer func() { act.Source = actSrc }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		res, err := pub.PublishAction(ctx, act)
		assert.Nil(t, res)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Action Source "": must not be empty`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("SubscribeAction with invalid Filter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		actions, err := sub1.SubscribeAction(ctx, api.SubscriptionFilter{Type: "foo/bar"})
		assert.Nil(t, actions)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Action Type "foo/bar": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})

	var actCb api.ActionWithCallback
	var actRes api.ActionResult

	t.Run("Publish-SubscribeAction", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		actions1, err1 := sub1.SubscribeAction(ctx1, api.SubscriptionFilter{Type: act.Type})
		assert.NotNil(t, actions1)
		assert.NoError(t, err1)

		ctx2, cancel2 := context.WithCancel(context.Background())
		actions2, err2 := sub2.SubscribeAction(ctx2, api.SubscriptionFilter{Type: act.Type})
		assert.NotNil(t, actions2)
		assert.NoError(t, err2)

		ctxRes1, cancelRes1 := context.WithCancel(context.Background())
		results1, errRes := pub.PublishAction(ctxRes1, act)
		assert.NotNil(t, results1)
		assert.NoError(t, errRes)

		actCb1 := <-actions1
		actCb2 := <-actions2
		assert.Equal(t, act, actCb1.Action)
		assert.Equal(t, act, actCb2.Action)

		actRes1 := api.ActionResult{
			Context:        "sub1",
			Data:           actCb1.Params[:1],
			SequenceNumber: 0,
		}
		assert.NoError(t, actCb1.Callback(actRes1))

		actCb = actCb1
		actRes = actRes1

		time.Sleep(200 * time.Millisecond)

		actRes21 := api.ActionResult{
			Context:        "sub2",
			Data:           actCb2.Params[len(actCb2.Params)-1:],
			SequenceNumber: 1,
		}
		assert.NoError(t, actCb2.Callback(actRes21))

		assert.Equal(t, actRes1, <-results1)
		assert.Equal(t, actRes21, <-results1)

		actRes22 := api.ActionResult{
			Context:        "sub2",
			Data:           actCb2.Params[len(actCb2.Params)-1:],
			SequenceNumber: 2,
		}
		assert.NoError(t, actCb2.Callback(actRes22))

		assert.Equal(t, actRes22, <-results1)

		cancel1() // unsubscribe is asyn, await close before publishing
		<-actions1

		act.Id = "43"
		ctxRes2, cancelRes2 := context.WithCancel(context.Background())
		results2, errRes2 := pub.PublishAction(ctxRes2, act, api.ScopeCom)
		assert.NotNil(t, results2)
		assert.NoError(t, errRes2)

		actCb3 := <-actions2
		assert.Equal(t, act, actCb3.Action)
		actRes23 := api.ActionResult{
			Context:        "sub2",
			Data:           actCb3.Params[:1],
			SequenceNumber: 0,
		}
		assert.NoError(t, actCb3.Callback(actRes23))
		assert.Equal(t, actRes23, <-results2)

		actRes22.SequenceNumber = -3 // final result
		assert.NoError(t, actCb2.Callback(actRes22))
		assert.Equal(t, actRes22, <-results1)

		cancel2()
		cancelRes1()
		cancelRes2()

		actRes22.SequenceNumber = 4
		assert.NoError(t, actCb2.Callback(actRes22))
		assert.Equal(t, api.ActionResult{}, <-results1) // zero value on closed channel

		actRes23.SequenceNumber = 1
		assert.NoError(t, actCb3.Callback(actRes23))
		assert.Equal(t, api.ActionResult{}, <-results2) // zero value on closed channel
	})

	t.Run("SubscribeAction with invalid Share", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		actions, err := sub1.SubscribeAction(ctx, api.SubscriptionFilter{Type: act.Type, Share: "+*"})
		assert.Nil(t, actions)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Action Share "+*": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})

	t.Run("Publish-SubscribeAction with Share", func(t *testing.T) {
		share := "group1"

		ctx1, cancel1 := context.WithCancel(context.Background())
		defer cancel1()
		actions1, err1 := sub1.SubscribeAction(ctx1, api.SubscriptionFilter{Type: act.Type, Share: share})
		assert.NotNil(t, actions1)
		assert.NoError(t, err1)

		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()
		actions2, err2 := sub2.SubscribeAction(ctx2, api.SubscriptionFilter{Type: act.Type, Share: share})
		assert.NotNil(t, actions2)
		assert.NoError(t, err2)

		ctxRes, cancelRes := context.WithCancel(context.Background())
		defer cancelRes()
		results1, errRes := pub.PublishAction(ctxRes, act)
		assert.NotNil(t, results1)
		assert.NoError(t, errRes)

		rcvCnt := 0
	loop:
		for {
			select {
			case actCb1 := <-actions1:
				assert.Equal(t, act, actCb1.Action)
				rcvCnt++
				assert.NoError(t, actCb1.Callback(actRes))
			case actCb2 := <-actions2:
				assert.Equal(t, act, actCb2.Action)
				rcvCnt++
				assert.NoError(t, actCb2.Callback(actRes))
			case <-time.After(1 * time.Second):
				assert.Equalf(t, 1, rcvCnt, "shared subscription received %d actions", rcvCnt)
				break loop
			}
		}

		assert.Equal(t, actRes, <-results1)
	})

	qry := api.Query{
		Type:   "partsCount",
		Id:     "42",
		Source: "pub",
		Data: []byte(`{
            "query": "PartsCount($category Category) { parts(category: $category) { count }}",
            "operationName": "PartsCount",
            "variables": { "category": "foo" }
        }`),
		DataContentType: "application/graphql",
	}

	t.Run("PublishQuery with invalid Type", func(t *testing.T) {
		qryType := qry.Type
		qry.Type = "+-*"
		defer func() { qry.Type = qryType }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		res, err := pub.PublishQuery(ctx, qry)
		assert.Nil(t, res)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Query Type "+-*": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("PublishQuery with empty Type", func(t *testing.T) {
		qryType := qry.Type
		qry.Type = ""
		defer func() { qry.Type = qryType }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		res, err := pub.PublishQuery(ctx, qry)
		assert.Nil(t, res)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Query Type "": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("PublishQuery with empty Id", func(t *testing.T) {
		qryId := qry.Id
		qry.Id = ""
		defer func() { qry.Id = qryId }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		res, err := pub.PublishQuery(ctx, qry)
		assert.Nil(t, res)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Query Id "": must not be empty`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("PublishQuery with empty Source", func(t *testing.T) {
		qrySrc := qry.Source
		qry.Source = ""
		defer func() { qry.Source = qrySrc }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		res, err := pub.PublishQuery(ctx, qry)
		assert.Nil(t, res)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Query Source "": must not be empty`)
			assert.False(t, services.IsRetryable(err))
		}
	})
	t.Run("SubscribeQuery with invalid Filter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		queries, err := sub1.SubscribeQuery(ctx, api.SubscriptionFilter{Type: "foo/bar"})
		assert.Nil(t, queries)
		if assert.Error(t, err) {
			assert.EqualError(t, err, `invalid Query Type "foo/bar": must only contain characters 0-9, a-z, A-Z, dot, hyphen, or underscore`)
			assert.False(t, services.IsRetryable(err))
		}
	})

	var qryCb api.QueryWithCallback
	var qryRes api.QueryResult

	t.Run("Publish-SubscribeQuery", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		queries1, err1 := sub1.SubscribeQuery(ctx1, api.SubscriptionFilter{Type: qry.Type})
		assert.NotNil(t, queries1)
		assert.NoError(t, err1)

		ctx2, cancel2 := context.WithCancel(context.Background())
		queries2, err2 := sub2.SubscribeQuery(ctx2, api.SubscriptionFilter{Type: qry.Type})
		assert.NotNil(t, queries2)
		assert.NoError(t, err2)

		ctxRes, cancelRes := context.WithCancel(context.Background())
		results, err := pub.PublishQuery(ctxRes, qry)
		assert.NotNil(t, results)
		assert.NoError(t, err)

		qryCb1 := <-queries1
		qryCb2 := <-queries2
		assert.Equal(t, qry, qryCb1.Query)
		assert.Equal(t, qry, qryCb2.Query)

		qryRes1 := api.QueryResult{
			Context:        "sub1",
			Data:           []byte(`{ "data": { "parts": { "count" : 42 }}}`),
			SequenceNumber: 0,
		}
		assert.NoError(t, qryCb1.Callback(qryRes1))

		qryCb = qryCb1
		qryRes = qryRes1

		time.Sleep(200 * time.Millisecond)

		qryRes21 := api.QueryResult{
			Context:        "sub2",
			Data:           []byte(`{ "data": { "parts": { "count" : 43 }}}`),
			SequenceNumber: 1,
		}
		assert.NoError(t, qryCb2.Callback(qryRes21))

		assert.Equal(t, qryRes1, <-results)
		assert.Equal(t, qryRes21, <-results)

		qryRes22 := api.QueryResult{
			Context:        "sub2",
			Data:           []byte(`{ "data": { "parts": { "count" : 44 }}}`),
			SequenceNumber: -2, // final result
		}
		assert.NoError(t, qryCb2.Callback(qryRes22))
		assert.Equal(t, qryRes22, <-results)

		cancel1()
		cancel2()
		cancelRes()

		qryRes22.SequenceNumber = 3
		assert.NoError(t, qryCb2.Callback(qryRes22))
		assert.Equal(t, api.QueryResult{}, <-results) // zero value on closed channel
	})

	t.Run("Close all pub-subs", func(t *testing.T) {
		testdata.CloseDda(pub)
		testdata.CloseDda(sub1)
		testdata.CloseDda(sub2)
	})

	t.Run("Publish-SubscribeEvent on closed pub-sub", func(t *testing.T) {
		err := pub.PublishEvent(evt)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		events, err := sub1.SubscribeEvent(ctx, api.SubscriptionFilter{Type: evt.Type})
		assert.Nil(t, events)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))
	})
	t.Run("Publish-SubscribeAction on closed pub-sub", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		res, err := pub.PublishAction(ctx, act)
		assert.Nil(t, res)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))

		actions, err := sub1.SubscribeAction(ctx, api.SubscriptionFilter{Type: act.Type})
		assert.Nil(t, actions)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))

		err = actCb.Callback(actRes)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))
	})
	t.Run("Publish-SubscribeQuery on closed pub-sub", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		res, err := pub.PublishQuery(ctx, qry, api.ScopeCom)
		assert.Nil(t, res)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))

		actions, err := sub1.SubscribeQuery(ctx, api.SubscriptionFilter{Type: qry.Type})
		assert.Nil(t, actions)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))

		err = qryCb.Callback(qryRes)
		assert.Error(t, err)
		assert.False(t, services.IsRetryable(err))
	})

}

// Runs all benchmarks on the given communication service.
func RunBenchComService(b *testing.B, cluster string, comSrv config.ConfigComService) {
	// This benchmark with setup will not be measured itself and called once for
	// each service with b.N=1

	pub, err := testdata.OpenDda(cluster, "pub", comSrv)
	if err != nil {
		b.FailNow()
	}
	sub, err := testdata.OpenDda(cluster, "sub", comSrv)
	if err != nil {
		b.FailNow()
	}

	pubDataPrefix := "Hello from pub "
	pubData := []byte(pubDataPrefix + pub.Identity().Id)

	evt := api.Event{
		Type:   "hello",
		Id:     "42",
		Source: "pub",
		Data:   pubData,
	}

	defer testdata.CloseDda(sub)
	defer testdata.CloseDda(pub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := sub.SubscribeEvent(ctx, api.SubscriptionFilter{Type: evt.Type})
	if err != nil {
		b.FailNow()
	}

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Pub-Sub One-way", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			err = pub.PublishEvent(evt)
			if err != nil {
				b.FailNow()
			}
			<-events
		}
	})

	act := api.Action{
		Type:   "echo",
		Id:     "42",
		Source: "pub",
		Params: []byte{1, 2, 3, 4, 5},
	}

	actionsCb, errAct := sub.SubscribeAction(ctx, api.SubscriptionFilter{Type: act.Type})
	if errAct != nil {
		b.FailNow()
	}

	go func() {
		for {
			if acb, ok := <-actionsCb; ok {
				err = acb.Callback(api.ActionResult{
					Context:        "sub",
					Data:           acb.Params,
					SequenceNumber: 0,
				})
				if err != nil {
					break
				}
			} else {
				break
			}
		}
	}()

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Pub-Sub Two-way", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			res, err := pub.PublishAction(ctx, act)
			if err != nil {
				b.FailNow()
			}
			<-res
		}
	})
}
