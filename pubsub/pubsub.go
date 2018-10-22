package pubsub

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"chainspace.io/prototype/freeport"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
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
	conns     []net.Conn
}

type Payload struct {
	ObjectID string `json:"object_id"`
	Success  bool   `json:"success"`
}

func (s *Server) handleConnection(conn net.Conn) {
	// may need to init with block number or sumbthing in the future
	s.conns = append(s.conns, conn)
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

func (s *Server) Publish(objectID string, success bool) {
	payload := Payload{
		ObjectID: objectID,
		Success:  success,
	}
	b, _ := json.Marshal(&payload)
	for _, v := range s.conns {

	}
}
func (c *Conn) writePayload(payload []byte, timeout time.Duration) error {
	size := len(payload)
	buf := c.buf[:4]
	binary.LittleEndian.PutUint32(buf, uint32(size))
	need := 4
	for need > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(timeout))
		n, err := c.conn.Write(buf)
		if err != nil {
			c.conn.Close()
			return err
		}
		buf = buf[n:]
		need -= n
	}
	for size > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(timeout))
		n, err := c.conn.Write(payload)
		if err != nil {
			c.conn.Close()
			return err
		}
		payload = payload[n:]
		size -= n
	}
	return nil
}

func announceMDNS(networkID string, nodeID uint64, port int) error {
	instance := fmt.Sprintf("_%d", nodeID)
	service := fmt.Sprintf("_%s_pubsub._chainspace", strings.ToLower(networkID))
	_, err := zeroconf.Register(instance, service, "local.", port, nil, nil)
	return err
}

func New(cfg *Config) (*Server, error) {
	var port int
	var mdns bool
	if cfg.Port == nil {
		mdns = true
		port, err := freeport.TCP("")
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
		conns:     []net.Conn{},
	}
	go srv.listen(ln)

	return srv, nil
}
