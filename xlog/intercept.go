package log

import (
	"time"
)

type intercept struct {
}

func (i intercept) Write(p []byte) (n int, err error) {
	if len(p) > 0 && p[len(p)-1] == '\n' {
		p = p[:len(p)-1]
	}
	textLog(entry{
		level: InfoLevel,
		text:  string(p),
		time:  time.Now().UTC(),
	})
	return len(p), nil
}
