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

	root, err := ensureRootDir()
	if err != nil {
		log.Fatal(err)
	}

	netDir := filepath.Join(root, *networkID)
	createUnlessExists(netDir)

	peers := map[uint64]*config.Peer{}

	for i := 1; i <= *nodeCount; i++ {

		log.Infof("Generating %s node %d", *networkID, i)

		dirName := fmt.Sprintf("node-%d", i)
		nodeDir := filepath.Join(netDir, dirName)
		createUnlessExists(nodeDir)

		// Create keypair.yaml
		keypair, err := genKey(filepath.Join(nodeDir, "keypair.yaml"))
		if err != nil {
			log.Fatal(err)
		}

		address := fmt.Sprintf("%s:%d", *ip, *portOffset+i)
		createNodeYaml(address, &i, nodeDir)

		peers[uint64(i)] = &config.Peer{
			Address: address,
			PubKey: &config.PublicKey{
				Algorithm: "ed25519",
				Value:     b32.EncodeToString(keypair.PublicKey()),
			},
		}
	}

	if err = writeYAML(filepath.Join(netDir, "peers.yaml"), peers); err != nil {
		log.Fatal(err)
	}
}

func createNodeYaml(address string, i *int, nodeDir string) {
	cfg := &config.Node{
		Address: address,
		ID:      uint64(*i),
	}
	if err := writeYAML(filepath.Join(nodeDir, "node.yaml"), cfg); err != nil {
		log.Fatal(err)
	}
}
