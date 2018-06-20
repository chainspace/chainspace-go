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
	bindAll := opts.Flags("--bind-all").Bool("override --host-ip and bind to all interfaces instead")
	configRoot := opts.Flags("--config-root").Label("PATH").String("path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	hostIP := opts.Flags("--host-ip").Label("IP").String("the default ip to bind the nodes [127.0.0.1]")
	networkName := opts.Flags("--network-name").Label("IDENT").String("the name of the network [testnet]")
	runtimeRoot := opts.Flags("--runtime-root").Label("PATH").String("path to the runtime root directory [~/.chainspace]")
	shardCount := opts.Flags("--shard-count").Label("N").Int("number of shards in the network [3]")
	shardSize := opts.Flags("--shard-size").Label("N").Int("number of nodes in each shard [4]")
	opts.Parse(args)

	if err := ensureDir(*configRoot); err != nil {
		log.Fatal(err)
	}

	netDir := filepath.Join(*configRoot, *networkName)
	createUnlessExists(netDir)

	buf := make([]byte, 36)
	if _, err := rand.Read(buf); err != nil {
		log.Fatal(err)
	}

	networkID := b32.EncodeToString(buf)
	peers := map[uint64]*config.Peer{}
	shard := &config.Shard{
		Count: *shardCount,
		Size:  *shardSize,
	}

	network := &config.Network{
		ID:        networkID,
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

		log.Infof("Generating %s node %d", *networkName, i)

		nodeID := uint64(i)
		dirName := fmt.Sprintf("node-%d", i)
		nodeDir := filepath.Join(netDir, dirName)
		createUnlessExists(nodeDir)

		// Create keys.yaml
		signingKey, cert, err := genKeys(filepath.Join(nodeDir, "keys.yaml"), *networkName, nodeID)
		if err != nil {
			log.Fatal(err)
		}

		// Create node.yaml
		cfg := &config.Node{
			Announce:         []string{"mdns"},
			BootstrapMDNS:    true,
			HostIP:           ip,
			RuntimeDirectory: filepath.Join(*runtimeRoot, *networkName, dirName),
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
