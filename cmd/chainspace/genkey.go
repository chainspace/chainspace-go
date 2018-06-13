package main

import (
	"log"
	"os"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto"
	"github.com/tav/golly/optparse"
	"gopkg.in/yaml.v2"
)

func cmdGenKey(args []string, usage string) {
	opts := optparse.New("Usage: chainspace genkey\n\n  " + usage + "\n")
	opts.Parse(args)
	keypair, err := crypto.GenKeyPair("ed25519")
	if err != nil {
		log.Fatalf("ERROR: could not generate keypair: %s", err)
	}
	f, err := os.Create("keypair.yaml")
	if err != nil {
		log.Fatalf("ERROR: could not create keypair.yaml: %s", err)
	}
	enc := yaml.NewEncoder(f)
	cfg := config.KeyPair{
		Algorithm: keypair.Algorithm(),
		PubKey:    string(keypair.PublicKey()),
		PrivKey:   string(keypair.PrivateKey()),
	}
	err = enc.Encode(cfg)
	if err != nil {
		log.Fatalf("ERROR: could not write data to keypair.yaml: %s", err)
	}
	f.Close()
	log.Printf("Generated keypair successfully written to keypair.yaml")
}
