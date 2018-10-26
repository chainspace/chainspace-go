package freeport

import (
	"testing"
)

func TestTCP(t *testing.T) {
	port, err := TCP("localhost")
	if err != nil {
		t.Errorf("unable to get a free port: %s", err)
		return
	}
	if port < 0 || port > 65535 {
		t.Errorf("got an invalid port number: %d", port)
	}
}

func TestUDP(t *testing.T) {
	port, err := UDP("localhost")
	if err != nil {
		t.Errorf("unable to get a free port: %s", err)
		return
	}
	if port < 0 || port > 65535 {
		t.Errorf("got an invalid port number: %d", port)
	}
}
