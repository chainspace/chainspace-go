package main

import (
	"fmt"
	"path/filepath"

	"chainspace.io/prototype/config"
	"github.com/tav/golly/log"
)

func cmdGenTestnet(args []string, usage string) {

	opts := newOpts("testnet OPTIONS", usage)
	ip := opts.Flags("-i", "--ip-address").Label("IP").String("base IP address to use for nodes [127.0.0.1]")
	networkID := opts.Flags("-n", "--network-id").Label("ID").String("ID of the network [testnet]")
	nodeCount := opts.Flags("-c", "--count").Int("number of nodes to instantiate [4]")
	portOffset := opts.Flags("-p", "--port-offset").Int("starting port offset for node addresses [9000]")
	opts.Parse(args)

	if err := ensureRootDir(); err != nil {
		log.Fatal(err)
	}

	netDir := filepath.Join(rootDir, *networkID)
	createUnlessExists(netDir)

	peers := map[uint64]*config.Peer{}

	for i := 1; i <= *nodeCount; i++ {

		log.Infof("Generating %s node %d", *networkID, i)

		dirName := fmt.Sprintf("node-%d", i)
		nodeDir := filepath.Join(netDir, dirName)
		createUnlessExists(nodeDir)

		// Create keys.yaml
		signingKey, cert, err := genKeys(filepath.Join(nodeDir, "keys.yaml"))
		if err != nil {
			log.Fatal(err)
		}

		// Create node.yaml
		address := fmt.Sprintf("%s:%d", *ip, *portOffset+i)
		cfg := &config.Node{
			Address: address,
			ID:      uint64(i),
		}
		if err := writeYAML(filepath.Join(nodeDir, "node.yaml"), cfg); err != nil {
			log.Fatal(err)
		}

		peers[uint64(i)] = &config.Peer{
			Address: address,
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

	if err := writeYAML(filepath.Join(netDir, "peers.yaml"), peers); err != nil {
		log.Fatal(err)
	}

}
