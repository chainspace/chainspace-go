package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/transactor"

	"github.com/tav/golly/optparse"
)

func getRequiredParams(
	opts *optparse.Parser, args []string) (net string, cmd string, payload string) {
	params := opts.Parse(args)
	if len(params) != 3 {
		opts.PrintUsage()
		os.Exit(1)
	}
	net = params[0]
	if net == "" {
		log.Fatal("Network name cannot be empty")
	}
	cmd = params[1]
	if net == "" {
		log.Fatal("Command cannot be empty")
	}
	payload = params[2]
	if net == "" {
		log.Fatal("Payload cannot be empty")
	}
	return net, cmd, payload
}

func cmdTransactor(args []string, usage string) {
	opts := newOpts("transactor NETWORK_NAME COMMAND COMMAND_PAYLOAD_PATH [OPTIONS]", usage)
	configRoot := opts.Flags("-c", "--config-root").Label("PATH").String("path to the chainspace root directory [$HOME/.chainspace]", defaultRootDir())
	nodeID := opts.Flags("-n", "--node-id").Label("NODE_ID").Int("node to send the command to (if not specified send the command to a random node)", 0)
	networkName, cmd, payloadPath := getRequiredParams(opts, args)

	_, err := os.Stat(*configRoot)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Could not find the Chainspace root directory at %s", *configRoot)
		}
		log.Fatalf("Unable to access the Chainspace root directory at %s: %s", *configRoot, err)
	}

	netPath := filepath.Join(*configRoot, networkName)
	netCfg, err := config.LoadNetwork(filepath.Join(netPath, "network.yaml"))
	if err != nil {
		log.Fatalf("Could not load network.yaml: %s", err)
	}

	if *nodeID == 0 {
		rand.Seed(time.Now().UTC().UnixNano())
		*nodeID = 1 + (rand.Int() % (len(netCfg.SeedNodes)))
	}

	payload, err := ioutil.ReadFile(payloadPath)
	if err != nil {
		log.Fatalf("Unable to read payload file: %v", err)
	}

	cfg := &transactor.Config{
		NetworkName:   networkName,
		NetworkConfig: *netCfg,
		NodeID:        uint64(*nodeID),
	}

	transactorClient, err := transactor.New(cfg)
	if err != nil {
		log.Fatalf("Unable to create transactor.Client")
	}

	switch cmd {
	case "transaction":
		tx := transactor.Transaction{}
		err := json.Unmarshal(payload, &tx)
		if err != nil {
			log.Fatalf("Invalid payload format for transaction: %v", err)
		}
		err = transactorClient.SendTransaction(&tx)
		if err != nil {
			log.Fatalf("Unable to send transaction: %v", err)
		}
	case "query":
		log.Fatalf("Unavailable command: %s", cmd)
	default:
		log.Fatalf("Invalid command name: %s", cmd)
	}
}
