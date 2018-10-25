package pubsub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"chainspace.io/prototype/freeport"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/pubsub/internal"
	"github.com/grandcat/zeroconf"
)

type Config struct {
	Port      *int
	NetworkID string
	NodeID    uint64
}

type Server struct {
	port      int
	networkID string
	nodeID    uint64
	conns     map[string]*internal.Conn
	mu        sync.Mutex
}

func (s *Server) Close() {
	for _, v := range s.conns {
		v.Close()
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	// may need to init with block number or sumbthing in the future
	s.conns[conn.RemoteAddr().String()] = internal.NewConn(conn)
}

func (s *Server) listen(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Error("pubsub: Could not accept new connections", fld.Err(err))
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) Publish(objectID []byte, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Error("sending object id", log.String("id", base64.StdEncoding.EncodeToString(objectID)))
	payload := internal.Payload{
		ObjectID: base64.StdEncoding.EncodeToString(objectID),
		Success:  success,
		NodeID:   s.nodeID,
	}
	b, _ := json.Marshal(&payload)
	badconns := []string{}
	for addr, c := range s.conns {
		if err := c.Write(b, 5*time.Second); err != nil {
			log.Error("unable to publish objectID", fld.Err(err))
			badconns = append(badconns, addr)
		}
	}
	// remove badconns
	for _, addr := range badconns {
		s.conns[addr].Close()
		delete(s.conns, addr)
	}
}

func announceMDNS(networkID string, nodeID uint64, port int) error {
	log.Error("ANNOUNCE MDNS PUBSUB")
	instance := fmt.Sprintf("_%d", nodeID)
	service := fmt.Sprintf("_%s_pubsub._chainspace", strings.ToLower(networkID))
	_, err := zeroconf.Register(instance, service, "local.", port, nil, nil)
	return err
}

func New(cfg *Config) (*Server, error) {
	var (
		port int
		mdns bool
		err  error
	)
	if cfg.Port == nil || *cfg.Port == 0 {
		mdns = true
		port, err = freeport.TCP("")
		if err != nil {
			return nil, err
		}
	} else {
		port = *cfg.Port
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		return nil, err
	}

	if mdns {
		if err := announceMDNS(cfg.NetworkID, cfg.NodeID, port); err != nil {
			return nil, err
		}

	}
	srv := &Server{
		port:      port,
		networkID: cfg.NetworkID,
		nodeID:    cfg.NodeID,
		conns:     map[string]*internal.Conn{},
	}
	go srv.listen(ln)

	return srv, nil
}
