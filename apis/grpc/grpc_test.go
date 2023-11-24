// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package grpc

import (
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/stretchr/testify/assert"
)

func TestGrpc(t *testing.T) {
	s := &grpcServer{}
	t.Run("openWebServer returns nil if gRPC-Web API is disabled", func(t *testing.T) {
		cfg := config.New()
		cfg.Apis.GrpcWeb.Disabled = true
		assert.Nil(t, s.openWebServer(s.srv, cfg))
	})
	t.Run("makeHttpOriginFunc all origins allowed", func(t *testing.T) {
		f1 := s.makeHttpOriginFunc(nil)
		f2 := s.makeHttpOriginFunc([]string{})
		assert.NotNil(t, f1)
		assert.NotNil(t, f2)
		assert.True(t, f2("any"))
		assert.True(t, f1("https://example.org"))
		assert.True(t, f1("https://awesome.com"))
		assert.True(t, f2("https://example.org"))
		assert.True(t, f2("https://awesome.com"))
	})
	t.Run("makeHttpOriginFunc configured origins allowed", func(t *testing.T) {
		f := s.makeHttpOriginFunc([]string{"https://example.org", "https://awesome.com"})
		assert.NotNil(t, f)
		assert.False(t, f("any"))
		assert.True(t, f("https://example.org"))
		assert.True(t, f("https://awesome.com"))
		assert.False(t, f("https://Example.org"))
		assert.False(t, f("https://Awesome.org"))
		assert.False(t, f("https://example.com"))
		assert.False(t, f("https://awesome.org"))
	})
}
