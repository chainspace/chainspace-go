package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"chainspace.io/chainspace-go/pubsub"
	"chainspace.io/chainspace-go/pubsub/internal"
	"github.com/gorilla/websocket"
)

type service struct {
	ctx       context.Context
	pbService pubsub.Server
	chans     map[string]*chanCloser
	mu        sync.Mutex // locks chans
}

type Service interface {
	Websocket(string, WriteMessageCloser) (int, error)
	Callback(p internal.Payload)
}

type chanCloser struct {
	C     chan internal.Payload
	close bool
	mu    sync.Mutex
}

func (c *chanCloser) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.close = true
}

func NewService(ctx context.Context, pbService pubsub.Server) Service {
	srv := &service{
		ctx:       ctx,
		pbService: pbService,
		chans:     map[string]*chanCloser{},
	}
	pbService.RegisterNotifier(srv.Callback)
	return srv
}

func (s *service) Callback(p internal.Payload) {
	go func(p internal.Payload) {
		s.mu.Lock()
		defer s.mu.Unlock()
		doneConn := []string{}
		for k, ch := range s.chans {
			ch.mu.Lock()
			if ch.close == true {
				close(ch.C)
				doneConn = append(doneConn, k)
			} else {
				ch.C <- p
			}
			ch.mu.Unlock()
		}
		for _, v := range doneConn {
			delete(s.chans, v)
		}
	}(p)
}

func (s *service) addChan(cltID string, c *chanCloser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chans[cltID] = c
}

func (s *service) Websocket(cltID string, conn WriteMessageCloser) (int, error) {
	ch := &chanCloser{
		C: make(chan internal.Payload, 100),
	}
	s.addChan(cltID, ch)
	for {
		select {
		case <-s.ctx.Done():
			ch.Close()
			if err := conn.Close(); err != nil {
				return http.StatusInternalServerError, err
			}
			return http.StatusOK, s.ctx.Err()
		case p := <-ch.C:
			msg, err := json.Marshal(p)
			if err != nil {
				ch.Close()
				return http.StatusInternalServerError, err
			}
			err = conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				ch.Close()
				return http.StatusInternalServerError, err
			}
		}
	}
}
