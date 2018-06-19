package main

import (
	"crypto/rand"
	"fmt"
	"net"
	"path/filepath"

	"chainspace.io/prototype/config"
	"github.com/tav/golly/log"
)

func cmdGenTestnet(args []string, usage string) {

	opts := newOpts("gentestnet [OPTIONS]", usage)
	count := opts.Flags("-c", "--count").Label("N").Int("number of nodes to instantiate [4]")
	ip := opts.Flags("-i", "--ip").Label("ADDR").String("default ip to bind the nodes [127.0.0.1]")
	name := opts.Flags("-n", "--name").Label("IDENT").String("the name of the network [testnet]")
	rootDir := opts.Flags("-r", "--root").Label("DIR").String("path to the chainspace root directory", defaultRootDir())
	opts.Parse(args)

	if err := ensureDir(*rootDir); err != nil {
		log.Fatal(err)
	}

	netDir := filepath.Join(*rootDir, *name)
	createUnlessExists(netDir)

	buf := make([]byte, 36)
	if _, err := rand.Read(buf); err != nil {
		log.Fatal(err)
	}

	networkID := b32.EncodeToString(buf)
	peers := map[uint64]*config.Peer{}
	network := &config.Network{
		ID:        networkID,
		Shards:    1,
		SeedNodes: peers,
	}

	if *ip != "" && net.ParseIP(*ip) == nil {
		log.Fatalf("Could not parse the given IP address: %q", *ip)
	}

	for i := 1; i <= *count; i++ {

		log.Infof("Generating %s node %d", *name, i)

		nodeID := uint64(i)
		dirName := fmt.Sprintf("node-%d", i)
		nodeDir := filepath.Join(netDir, dirName)
		createUnlessExists(nodeDir)

		// Create keys.yaml
		signingKey, cert, err := genKeys(filepath.Join(nodeDir, "keys.yaml"), *name, nodeID)
		if err != nil {
			log.Fatal(err)
		}

		// Create node.yaml
		cfg := &config.Node{
			Bootstrap: "mdns",
			HostIP:    *ip,
		}
		if err := writeYAML(filepath.Join(nodeDir, "node.yaml"), cfg); err != nil {
			log.Fatal(err)
		}

		peers[nodeID] = &config.Peer{
			SigningKey: &config.PeerKey{
				Type:  signingKey.Algorithm().String(),
				Value: b32.EncodeToString(signingKey.PublicKey().Value()),
			},
			TransportCert: &config.PeerKey{
				Type:  cert.Type.String(),
				Value: cert.Public,
			},
		}

	}

	if err := writeYAML(filepath.Join(netDir, "network.yaml"), network); err != nil {
		log.Fatal(err)
	}

}
