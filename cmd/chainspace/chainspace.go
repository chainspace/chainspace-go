package main

import (
	"github.com/tav/golly/optparse"
)

func main() {
	cmds := map[string]func([]string, string){
		"genkey": cmdGenKey,
		"run":    cmdRun,
	}
	info := map[string]string{
		"genkey": "generate a new node keypair",
		"run":    "run the chainspace node",
	}
	optparse.Commands("chainspace", "0.0.1", cmds, info)
}
