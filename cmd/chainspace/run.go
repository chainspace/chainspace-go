package main

import (
	"fmt"

	"chainspace.io/prototype/config"
	"github.com/tav/golly/log"
)

func cmdRun(args []string, info string) {
	fmt.Println(">> Running chainspace!")
	cfg, err := config.ParseNode("node.yaml")
	if err != nil {
		log.Fatalf("Could not parse node.yaml: %s", err)
	}
	keypair, err := config.ParseKeyPair("keypair.yaml")
	if err != nil {
		log.Fatalf("Could not parse keypair.yaml: %s", err)
	}
	peers, err := config.ParsePeers("peers.yaml")
	if err != nil {
		log.Fatalf("Could not parse peers.yaml: %s", err)
	}
	log.Infof("NODE CONFIG: %v", cfg)
	log.Infof("KEYPAIR: %#v", keypair)
	log.Infof("PEERS: %v", peers)
}
