package main

import (
	"os"
	"strconv"

	"github.com/tav/golly/log"
)

func cmdInit(args []string, usage string) {
	opts := newOpts("init NODE_ID", usage)
	params := opts.Parse(args)
	if len(params) == 0 {
		opts.PrintUsage()
		os.Exit(1)
	}
	id, err := strconv.ParseUint(params[0], 10, 64)
	if err != nil {
		log.Fatalf("%q is not a valid node ID. It should be a number: %s", params[0], err)
	}
	_ = id
	log.Fatal("NOT IMPLEMENTED :)")
}
