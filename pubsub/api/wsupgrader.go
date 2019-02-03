package api

import (
	"io"
	"net/http"

	"github.com/gorilla/websocket"
)

const (
	readBufferSize  = 1024
	writeBufferSize = 1024
)

type WriteMessageCloser interface {
	io.Closer
	WriteMessage(int, []byte) error
}

type WSUpgrader interface {
	Upgrade(w http.ResponseWriter, r *http.Request) (WriteMessageCloser, error)
}

type upgrader struct{}

func (u upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (WriteMessageCloser, error) {
	var wsupgrader = websocket.Upgrader{
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
