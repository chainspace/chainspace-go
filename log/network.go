package log

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

var (
	pending chan entry
)

func netLog(e entry) {
	if netLevel > e.level {
		return
	}
	select {
	case pending <- e:
	default:
	}
}

func writeNetEntries(addr string) {
	var (
		backoff time.Duration
		bc      *bufio.Writer
		c       net.Conn
		e       entry
		err     error
	)
	buf := make([]byte, 8)
	pbuf := make([]byte, 4)
	maxBackoff := 5 * time.Second
	for {
		if e.level == 0 {
			e = <-pending
		}
		for c == nil {
			if backoff != maxBackoff {
				backoff += time.Second
			}
			c, err = net.Dial("tcp", addr)
			if err != nil {
				time.Sleep(backoff)
				continue
			} else {
				backoff = 0
				bc = bufio.NewWriter(c)
			}
		}
		buf = buf[:8]
		pbuf = pbuf[:4]
		buf = append(buf, byte(e.level))
		buf = writeUint32(buf, len(e.text))
		buf = append(buf, e.text...)
		flen := len(e.parfields) + len(e.fields)
		buf = writeUint32(buf, flen)
		if flen > 0 {
			if len(e.parfields) > 0 {
				buf, pbuf = writeNetFields(buf, pbuf, e.parfields)
			}
			if len(e.fields) > 0 {
				buf, pbuf = writeNetFields(buf, pbuf, e.fields)
			}
		}
		binary.LittleEndian.PutUint32(buf[4:], uint32(len(buf)-8))
		_, err = bc.Write(buf[4:])
		if err != nil {
			c = nil
			continue
		}
		binary.LittleEndian.PutUint32(pbuf, uint32(len(pbuf)-4))
		_, err = bc.Write(pbuf)
		if err != nil {
			c = nil
			continue
		}
	}
}

func writeNetFields(buf []byte, pbuf []byte, fields []Field) ([]byte, []byte) {
	for _, field := range fields {
		buf = writeUint32(buf, len(field.key))
		buf = append(buf, field.key...)
		idx := -1
		if field.key[0] == '@' {
			idx = len(buf)
			pbuf = append(pbuf, field.key...)
		}
		switch field.typ {
		case TypeBool:
			if field.ival == 1 {
				buf = append(buf, byte(TypeBool), 0x01)
			} else {
				buf = append(buf, byte(TypeBool), 0x00)
			}
		case TypeBools:
			buf = append(buf, byte(TypeBools))
			val := field.xval.([]bool)
			buf = writeUint32(buf, len(val))
			for _, v := range val {
				if v {
					buf = append(buf, 0x01)
				} else {
					buf = append(buf, 0x00)
				}
			}
		case TypeErr:
			buf = append(buf, byte(TypeErr))
			buf = writeUint32(buf, len(field.sval))
			buf = append(buf, field.sval...)
		case TypeString:
			buf = append(buf, byte(TypeString))
			buf = writeUint32(buf, len(field.sval))
			buf = append(buf, field.sval...)
		case TypeUint32:
			buf = append(buf, byte(TypeUint32))
			binary.LittleEndian.PutUint32(buf, uint32(field.ival))
			buf = append(buf, buf[:4]...)
		case TypeUint64:
			buf = append(buf, byte(TypeUint64))
			binary.LittleEndian.PutUint64(buf, uint64(field.ival))
			buf = append(buf, buf[:8]...)
		default:
			panic(fmt.Errorf("log: network encoding for field type not implemented: %s", field.typ))
		}
		if idx > -1 {
			pbuf = append(pbuf, buf[idx:]...)
		}
	}
	return buf, pbuf
}

func writeUint32(buf []byte, val int) []byte {
	binary.LittleEndian.PutUint32(buf, uint32(val))
	return append(buf, buf[:4]...)
}

func writeUint64(buf []byte, val int) []byte {
	binary.LittleEndian.PutUint64(buf, uint64(val))
	return append(buf, buf[:8]...)
}

// ToServer starts logging to the given server. It starts dropping backfilled
// entries beyond the given limit.
func ToServer(addr string, lvl level, limit int) {
	pending = make(chan entry, limit)
	netLevel = lvl
	updateMinLevels()
	go writeNetEntries(addr)
}
