package main

import (
	"fmt"
	"log"

	"chainspace.io/prototype/config"
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
	log.Printf("NODE CONFIG: %v", cfg)
	log.Printf("KEYPAIR: %#v", keypair)
	log.Printf("PEERS: %v", peers)
}
