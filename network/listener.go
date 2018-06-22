package network

import "github.com/lucas-clemente/quic-go"

type Listener struct {
	listener quic.Listener
}

func (l *Listener) Accept() (*Conn, error) {
	sess, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}

	c := &Conn{
		session: sess,
	}

	return c, nil
}

func (l *Listener) Close() {
	l.listener.Close()
}
