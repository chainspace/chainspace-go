package main

import (
	"encoding/base32"
	"fmt"
	"os"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto"
	"github.com/tav/golly/log"
	"gopkg.in/yaml.v2"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func genKey(path string) (*crypto.KeyPair, error) {
	keypair, err := crypto.GenKeyPair("ed25519")
	if err != nil {
		return nil, fmt.Errorf("could not generate keypair: %s", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg := config.KeyPair{
		Algorithm: keypair.Algorithm(),
		PubKey:    b32.EncodeToString(keypair.PublicKey()),
		PrivKey:   b32.EncodeToString(keypair.PrivateKey()),
	}
	enc := yaml.NewEncoder(f)
	err = enc.Encode(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not write data to %s: %s", path, err)
	}
	return keypair, nil
}

func cmdGenKey(args []string, usage string) {
	opts := newOpts("genkey OPTIONS", usage)
	path := opts.Flags("-o", "--output").Label("PATH").String("path to write the generated keypair [keypair.yaml]")
	opts.Parse(args)
	if _, err := genKey(*path); err != nil {
		log.Fatalf("ERROR: %s", err)
	}
	log.Infof("Generated keypair successfully written to %s", *path)
}
