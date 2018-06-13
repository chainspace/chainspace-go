package main

import (
	"fmt"
	"log"

	"chainspace.io/prototype/config"
)

func main() {
	fmt.Println(">> Running chainspace!")
	cfg, err := config.ParseNode("node.yaml")
	if err != nil {
		log.Fatalf("Could not parse node.yaml: %s", err)
	}
	privkey, err := config.ParsePrivkey("privkey.yaml")
	if err != nil {
		log.Fatalf("Could not parse privkey.yaml: %s", err)
	}
	log.Printf("NODE CONFIG: %#v", cfg)
	log.Printf("PRIVKEY: %#v", privkey)
}
