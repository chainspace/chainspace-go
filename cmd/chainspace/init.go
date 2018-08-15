package main

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"chainspace.io/prototype/log/fld"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
)

func cmdInit(args []string, usage string) {
	opts := newOpts("init NETWORK_NAME [OPTIONS]", usage)

	configRoot := opts.Flags("--config-root").Label("PATH").String("Path to the Chainspace root directory [~/.chainspace]", defaultRootDir())
	shardCount := opts.Flags("--shard-count").Label("N").Int("Number of shards in the network [3]")
	shardSize := opts.Flags("--shard-size").Label("N").Int("Number of nodes in each shard [4]")
	registryURL := opts.Flags("--registry-url").Label("URL").String("address of the registry to bootrasp / announce on the network", "")

	params := opts.Parse(args)

	if err := ensureDir(*configRoot); err != nil {
		log.Fatal("Could not ensure the existence of the root directory", fld.Err(err))
	}

	if len(params) < 1 {
		opts.PrintUsage()
		os.Exit(1)
	}

	networkName := params[0]
	netDir := filepath.Join(*configRoot, networkName)
	createUnlessExists(netDir)

	consensus := &config.Consensus{
		BlockLimit:      100 * config.MB,
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

	bootstrap := &config.Bootstrap{}
	announceURL := ""
	token := ""
	if len(*registryURL) > 0 {
		u, err := url.Parse(*registryURL)
		if err != nil || (!strings.HasPrefix(*registryURL, "https://") && !strings.HasPrefix(*registryURL, "http://")) {
			log.Fatal("the given url is not a valid http(s) url", log.String("url", *registryURL))
		}
		token = randSeq(10)
		u2 := *u
		u.Path = path.Join(u.Path, "contacts.list")
		bootstrap.URL = u.String()
		u2.Path = path.Join(u2.Path, "contacts.set")
		announceURL = u2.String()
	} else {
		bootstrap.MDNS = true
	}

	broadcast := &config.Broadcast{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
	}

	connections := &config.Connections{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logging := &config.Logging{
		ConsoleLevel: log.DebugLevel,
		FileLevel:    log.DebugLevel,
		FilePath:     "log/chainspace.log",
	}

	storage := &config.Storage{
		Type: "badger",
	}

	if ((3 * (*shardSize / 3)) + 1) != *shardSize {
		log.Fatal("The given --shard-size does not satisfy the 3f+1 requirement", fld.ShardSize(uint64(*shardSize)))
	}

	totalNodes := *shardCount * *shardSize
	for i := 1; i <= totalNodes; i++ {

		log.Info("Generating node", fld.NetworkName(networkName), fld.NodeID(uint64(i)))

		nodeID := uint64(i)
		dirName := fmt.Sprintf("node-%d", i)
		nodeDir := filepath.Join(netDir, dirName)
		createUnlessExists(nodeDir)

		// Create keys.yaml
		signingKey, cert, err := genKeys(filepath.Join(nodeDir, "keys.yaml"), networkName, nodeID)
		if err != nil {
			log.Fatal("Could not generate keys", fld.Err(err))
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
			Bootstrap:   bootstrap,
			Broadcast:   broadcast,
			Connections: connections,
			Logging:     logging,
			Storage:     storage,
			HTTP:        httpcfg,
			Token:       token,
		}

		if len(announceURL) > 0 {
			cfg.Announce = []string{announceURL}
		} else {
			cfg.Announce = []string{"mdns"}
		}

		if err := writeYAML(filepath.Join(nodeDir, "node.yaml"), cfg); err != nil {
			log.Fatal("Could not write to node.yaml", fld.Err(err))
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
		log.Fatal("Could not generate the Network ID", fld.Err(err))
	}

	network.ID = b32.EncodeToString(networkID)
	if err := writeYAML(filepath.Join(netDir, "network.yaml"), network); err != nil {
		log.Fatal("Could not write to network.yaml", fld.Err(err))
	}

}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return base64.StdEncoding.EncodeToString([]byte(string(b)))
}
