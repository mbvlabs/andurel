package server

import "testing"

func TestNewPreservesDefaultAndExplicitZeroTimeouts(t *testing.T) {
	defaultServer := New(t.Context(), "localhost", "8080", DevEnvironment, nil, nil)
	if defaultServer.srv.ReadTimeout <= 0 || defaultServer.srv.WriteTimeout <= 0 || defaultServer.srv.IdleTimeout <= 0 {
		t.Fatal("default server must have bounded HTTP timeouts")
	}
	zeroServer := New(t.Context(), "localhost", "8080", DevEnvironment, nil, nil, WithTimeouts(0, 0, 0))
	if zeroServer.srv.ReadTimeout != 0 || zeroServer.srv.WriteTimeout != 0 || zeroServer.srv.IdleTimeout != 0 {
		t.Fatal("explicit zero timeouts must disable the bounds")
	}
}
