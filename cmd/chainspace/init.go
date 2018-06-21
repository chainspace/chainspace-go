package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"chainspace.io/prototype/config"
	"github.com/tav/golly/log"
)

func cmdInit(args []string, usage string) {

	opts := newOpts("init NETWORK_NAME [OPTIONS]", usage)
	bindAll := opts.Flags("--bind-all").Bool("Override --host-ip and bind to all interfaces instead")
	configRoot := opts.Flags("--config-root").Label("PATH").String("Path to the Chainspace root directory [~/.chainspace]", defaultRootDir())
	hostIP := opts.Flags("--host-ip").Label("IP").String("The default ip to bind the nodes [127.0.0.1]")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("Path to the runtime root directory [~/.chainspace]")
	shardCount := opts.Flags("--shard-count").Label("N").Int("Number of shards in the network [3]")
	shardSize := opts.Flags("--shard-size").Label("N").Int("Number of nodes in each shard [4]")
	params := opts.Parse(args)

	if err := ensureDir(*configRoot); err != nil {
		log.Fatal(err)
	}

	if len(params) < 1 {
		opts.PrintUsage()
		os.Exit(1)
	}

	networkName := params[0]
	netDir := filepath.Join(*configRoot, networkName)
	createUnlessExists(netDir)

	peers := map[uint64]*config.Peer{}
	shard := &config.Shard{
		Count: *shardCount,
		Size:  *shardSize,
	}

	network := &config.Network{
		Shard:     shard,
		SeedNodes: peers,
	}

	ip := ""
	if !*bindAll {
		ip = *hostIP
	}
	if ip != "" && net.ParseIP(ip) == nil {
		log.Fatalf("Could not parse the given IP address: %q", ip)
	}

	if ((3 * (*shardSize / 3)) + 1) != *shardSize {
		log.Fatalf("The given --shard-size of %d does not satisfy the 3f+1 requirement", *shardSize)
	}

	totalNodes := *shardCount * *shardSize
	for i := 1; i <= totalNodes; i++ {

		log.Infof("Generating %s node %d", networkName, i)

		nodeID := uint64(i)
		dirName := fmt.Sprintf("node-%d", i)
		nodeDir := filepath.Join(netDir, dirName)
		createUnlessExists(nodeDir)

		// Create keys.yaml
		signingKey, cert, err := genKeys(filepath.Join(nodeDir, "keys.yaml"), networkName, nodeID)
		if err != nil {
			log.Fatal(err)
		}

		// Create node.yaml
		cfg := &config.Node{
			Announce:         []string{"mdns"},
			BootstrapMDNS:    true,
			HostIP:           ip,
			RuntimeDirectory: filepath.Join(*runtimeRoot, networkName, dirName),
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

	networkID, err := network.Hash()
	if err != nil {
		log.Fatalf("Could not generate the Network ID: %s", err)
	}

	network.ID = b32.EncodeToString(networkID)
	if err := writeYAML(filepath.Join(netDir, "network.yaml"), network); err != nil {
		log.Fatal(err)
	}

}
