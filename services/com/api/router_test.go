// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package api_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/coatyio/dda/services/com/api"
	"github.com/stretchr/testify/assert"
)

func TestRouter(t *testing.T) {
	r := api.NewRouter[api.Event, string]()
	assert.Equal(t, []string{}, r.GetTopics())

	f1 := api.RouteFilter[string]{Topic: "foo"}
	f2 := api.RouteFilter[string]{Topic: "foo", CorrelationId: "1"}
	f3 := api.RouteFilter[string]{Topic: "bar"}

	bndFuncCalled := func() error {
		return nil
	}
	bndFuncErrored := func() error {
		return fmt.Errorf("error in bndFunc")
	}
	onceCounter := 0
	bndFuncCalledOnce := func() error {
		onceCounter++
		return nil
	}

	ctx11, cancel11 := context.WithCancel(context.Background())
	rc11, err := r.Add(ctx11, f1, bndFuncCalled, bndFuncCalled, bndFuncCalledOnce)
	assert.Equal(t, 1, cap(rc11.ReceiveChan))
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo"}, r.GetTopics())
	assert.Equal(t, 0, onceCounter)

	ctx12, cancel12 := context.WithCancel(context.Background())
	rc12, err := r.Add(ctx12, f1, bndFuncCalled, bndFuncCalled, bndFuncCalledOnce, 4)
	assert.Equal(t, 4, cap(rc12.ReceiveChan))
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo"}, r.GetTopics())
	assert.Equal(t, 0, onceCounter)

	ctx21, cancel21 := context.WithCancel(context.Background())
	rc21, err := r.Add(ctx21, f2, bndFuncCalled, bndFuncCalled, bndFuncCalledOnce)
	assert.Equal(t, 1, cap(rc21.ReceiveChan))
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo"}, r.GetTopics())
	assert.Equal(t, 0, onceCounter)

	rc22, err := r.Add(context.Background(), f1, bndFuncCalled, bndFuncErrored, bndFuncCalledOnce)
	assert.Nil(t, rc22)
	assert.Error(t, err)
	assert.Equal(t, []string{"foo"}, r.GetTopics())
	assert.Equal(t, 0, onceCounter)

	rc22, err = r.Add(context.Background(), f3, bndFuncErrored, bndFuncCalled, bndFuncCalledOnce)
	assert.Nil(t, rc22)
	assert.Error(t, err)
	assert.Equal(t, []string{"foo"}, r.GetTopics())
	assert.Equal(t, 0, onceCounter)

	rc22, err = r.Add(context.Background(), f3, bndFuncCalled, bndFuncErrored, bndFuncCalledOnce)
	assert.Nil(t, rc22)
	assert.Error(t, err)
	assert.Equal(t, []string{"foo"}, r.GetTopics())
	assert.Equal(t, 1, onceCounter)

	onceCounter = 0

	evt1 := api.Event{Type: "evt1type", Id: "1", Source: "evt1", Data: []byte{42}}
	evt2 := api.Event{Type: "evt2type", Id: "2", Source: "evt2", Data: []byte{43}}
	evt3 := api.Event{Type: "evt3type", Id: "3", Source: "evt3", Data: []byte{44}}
	rc11Chan := rc11.ReceiveChan
	rc12Chan := rc12.ReceiveChan
	rc21Chan := rc21.ReceiveChan

	go func() {
		r.Dispatch(f1, evt1)
		r.Dispatch(f1, evt2)

		mf2 := make(chan api.Event, 1)
		mf2 <- evt3
		r.DispatchChan(f2, mf2)
		close(mf2)
	}()

	assert.Equal(t, evt1, <-rc11Chan)
	assert.Equal(t, evt1, <-rc12Chan)
	assert.Equal(t, evt2, <-rc11Chan)
	assert.Equal(t, evt2, <-rc12Chan)
	assert.Equal(t, evt3, <-rc21Chan)

	cancel11()
	assert.Equal(t, api.Event{}, <-rc11Chan) // closed
	assert.Equal(t, 0, onceCounter)          // unsubscribe not yet called as "foo" topic is still subscribed by rc12 and rc21

	cancel21()
	assert.Equal(t, api.Event{}, <-rc21Chan) // closed
	assert.Equal(t, 0, onceCounter)          // unsubscribe not yet called as "foo" topic still subscribed by rc12

	cancel12()
	assert.Equal(t, api.Event{}, <-rc12Chan) // closed
	assert.Equal(t, 1, onceCounter)          // unsubscribe called as "foo" topic is no longer subscribed

	assert.Equal(t, []string{}, r.GetTopics())

	r.Dispatch(f2, evt3)                     // no-op as context already canceled
	assert.Equal(t, api.Event{}, <-rc21Chan) // closed
	assert.Equal(t, 1, onceCounter)          // unsubcribe not called again

	cancel11()                               // no-op
	assert.Equal(t, api.Event{}, <-rc11Chan) // closed
	assert.Equal(t, 1, onceCounter)          // unsubcribe not called again

	cancel12()                               // no-op
	assert.Equal(t, api.Event{}, <-rc12Chan) // closed
	assert.Equal(t, 1, onceCounter)          // unsubcribe not called again

	cancel21()                               // no-op
	assert.Equal(t, api.Event{}, <-rc21Chan) // closed
	assert.Equal(t, 1, onceCounter)          // unsubcribe not called again
}
