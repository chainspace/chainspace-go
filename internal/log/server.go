package log // import "chainspace.io/chainspace-go/internal/log"

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/dgraph-io/badger"
)

// Server represents a TCP log server.
type Server struct {
	DB *badger.DB
}

func (s *Server) handle(c net.Conn) {
	r := bufio.NewReader(c)
	buf := make([]byte, 1024)
	buf4 := make([]byte, 4)
	// buf8 := make([]byte, 8)
	pbuf := make([]byte, 1024)
	for {
		total := 0
		for total < 4 {
			n, err := r.Read(buf4[total:])
			if err != nil {
				Error("Couldn't read from connection", Err(err))
				return
			}
			total += n
		}
		size := int(binary.LittleEndian.Uint32(buf4))
		if size > cap(buf) {
			buf = make([]byte, size)
		}
		total = 0
		for total < size {
			n, err := r.Read(buf[total:size])
			if err != nil {
				Error("Couldn't read from connection", Err(err))
				return
			}
			total += n
		}
		total = 0
		for total < 4 {
			n, err := r.Read(buf4[total:])
			if err != nil {
				Error("Couldn't read from connection", Err(err))
				return
			}
			total += n
		}
		size = int(binary.LittleEndian.Uint32(buf4))
		if size > cap(pbuf) {
			pbuf = make([]byte, size)
		}
		total = 0
		for total < size {
			n, err := r.Read(pbuf[total:size])
			if err != nil {
				Error("Couldn't read from connection", Err(err))
				return
			}
			total += n
		}
		fmt.Printf("PBUF: %q\n", pbuf)
		fmt.Printf(" BUF: %q\n", buf)
	}
}

// Run instantiates the TCP log server.
func (s *Server) Run(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		s.handle(c)
	}
}
