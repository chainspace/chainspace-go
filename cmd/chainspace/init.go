package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"go.uber.org/zap"
)

func cmdInit(args []string, usage string) {

	opts := newOpts("init NETWORK_NAME [OPTIONS]", usage)
	configRoot := opts.Flags("-c", "--config-root").Label("PATH").String("Path to the Chainspace root directory [~/.chainspace]", defaultRootDir())
	shardCount := opts.Flags("--shard-count").Label("N").Int("Number of shards in the network [3]")
	shardSize := opts.Flags("--shard-size").Label("N").Int("Number of nodes in each shard [4]")
	params := opts.Parse(args)

	if err := ensureDir(*configRoot); err != nil {
		log.Fatal("Could not ensure the existence of the root directory", zap.Error(err))
	}

	if len(params) < 1 {
		opts.PrintUsage()
		os.Exit(1)
	}

	networkName := params[0]
	netDir := filepath.Join(*configRoot, networkName)
	createUnlessExists(netDir)

	consensus := &config.Consensus{
		BlockLimit:      10 * config.MB,
		NonceExpiration: 30 * time.Second,
		RoundInterval:   time.Second,
		ViewTimeout:     15,
	}

	peers := map[uint64]*config.Peer{}
	shard := &config.Shard{
		Count: *shardCount,
		Size:  *shardSize,
	}

	network := &config.Network{
		Consensus:  consensus,
		MaxPayload: 128 * config.MB,
		Shard:      shard,
		SeedNodes:  peers,
	}

	bootstrap := &config.Bootstrap{
		MDNS: true,
	}

	broadcast := &config.Broadcast{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
	}

	connections := &config.Connections{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logging := &config.Logging{
		ConsoleOutput: true,
	}

	storage := &config.Storage{
		Type: "badger",
	}

	if ((3 * (*shardSize / 3)) + 1) != *shardSize {
		log.Fatal("The given --shard-size does not satisfy the 3f+1 requirement", zap.Int("shard.size", *shardSize))
	}

	totalNodes := *shardCount * *shardSize
	for i := 1; i <= totalNodes; i++ {

		log.Info("Generating node", zap.String("network.name", networkName), zap.Int("node.id", i))

		nodeID := uint64(i)
		dirName := fmt.Sprintf("node-%d", i)
		nodeDir := filepath.Join(netDir, dirName)
		createUnlessExists(nodeDir)

		// Create keys.yaml
		signingKey, cert, err := genKeys(filepath.Join(nodeDir, "keys.yaml"), networkName, nodeID)
		if err != nil {
			log.Fatal("Could not generate keys", zap.Error(err))
		}

		var httpcfg config.HTTP
		if i == 1 {
			httpport := 8080
			httpcfg = config.HTTP{
				Port:    &httpport,
				Enabled: true,
			}
		}
		// Create node.yaml
		cfg := &config.Node{
			Announce:    []string{"mdns"},
			Bootstrap:   bootstrap,
			Broadcast:   broadcast,
			Connections: connections,
			Logging:     logging,
			Storage:     storage,
			HTTP:        httpcfg,
		}

		if err := writeYAML(filepath.Join(nodeDir, "node.yaml"), cfg); err != nil {
			log.Fatal("Could not write to node.yaml", zap.Error(err))
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
		log.Fatal("Could not generate the Network ID", zap.Error(err))
	}

	network.ID = b32.EncodeToString(networkID)
	if err := writeYAML(filepath.Join(netDir, "network.yaml"), network); err != nil {
		log.Fatal("Could not write to network.yaml", zap.Error(err))
	}

}
