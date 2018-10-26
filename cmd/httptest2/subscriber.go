package main

import (
	"context"
	"fmt"
	"sync"

	"chainspace.io/prototype/pubsub/client"
)

type okcallback func(objectID string)

type subscriber struct {
	psclient  *client.Client
	nodeCount int
	// object id -> count
	results map[string]count
	mu      sync.Mutex
}

type count struct {
	i  int
	cb okcallback
}

func (s *subscriber) Subscribe(objectID string, cb okcallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[objectID] = count{0, cb}
}

func (s *subscriber) cb(nodeID uint64, objectID string, success bool, labels []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cnt, ok := s.results[objectID]; ok && success == true {
		cnt.i += 1
		s.results[objectID] = cnt
		if cnt.i >= s.nodeCount {
			cnt.cb(objectID)
		}
	} else {
		fmt.Printf("unexpected object id, id=%v success=%v\n", objectID, success)
	}
}

func NewSubscriber(ctx context.Context, mdns bool, nodeCount int) *subscriber {
	s := subscriber{
		nodeCount: nodeCount,
		results:   map[string]count{},
	}
	if mdns {
		pubsubAddresses = map[uint64]string{}
	}
	for k, v := range pubsubAddresses {
		fmt.Printf("%v -> %v\n", k, v)
	}
	cfg := client.Config{
		NetworkName: networkName,
		NodeAddrs:   pubsubAddresses,
		CB:          s.cb,
		Ctx:         ctx,
	}
	clt := client.New(&cfg)
	s.psclient = clt
	return &s
}
