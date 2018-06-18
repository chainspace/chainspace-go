package freeport // import "chainspace.io/prototype/freeport"

import (
	"net"
)

// TCP returns a free TCP port for use by a server.
func TCP(host string) (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", host+":0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// UDP returns a free UDP port for use by a server.
func UDP(host string) (int, error) {
	addr, err := net.ResolveUDPAddr("udp", host+":0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	port := l.LocalAddr().(*net.UDPAddr).Port
	l.Close()
	return port, nil
}
