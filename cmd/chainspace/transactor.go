package main

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/service/transactor/client"

	"github.com/tav/golly/optparse"
)

func getRequiredParams(
	opts *optparse.Parser, args []string) (net string, cmd string) {
	params := opts.Parse(args)
	if len(params) != 2 {
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
	return
}

func cmdTransactor(args []string, usage string) {
	opts := newOpts("transactor NETWORK_NAME COMMAND [OPTIONS]", usage)
	configRoot := opts.Flags("-c", "--config-root").Label("PATH").String("path to the chainspace root directory [$HOME/.chainspace]", defaultRootDir())
	payloadPath := opts.Flags("-p", "--payload-path").Label("PATH").String("path to the payload of the transaction to send", "")
	object := opts.Flags("-o", "--object").Label("OBJECT").String("an object to create in chainspace", "")
	key := opts.Flags("-k", "--key").Label("KEY").String("a key/identifier for an object stored in chainspace, e.g [42 54 67]", "")

	networkName, cmd := getRequiredParams(opts, args)

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

	cfg := &transactorclient.Config{
		NetworkName:   networkName,
		NetworkConfig: *netCfg,
	}

	transactorClient, err := transactorclient.New(cfg)
	if err != nil {
		log.Fatalf("Unable to create transactor.Client")
	}
	defer transactorClient.Close()

	switch cmd {
	case "transaction":
		if payloadPath == nil || len(*payloadPath) <= 0 {
			log.Fatalf("missing required payload path")
		}
		payload, err := ioutil.ReadFile(*payloadPath)
		if err != nil {
			log.Fatalf("Unable to read payload file: %v", err)
		}

		tx := transactorclient.ClientTransaction{}
		err = json.Unmarshal(payload, &tx)
		if err != nil {
			log.Fatalf("Invalid payload format for transaction: %v", err)
		}
		err = transactorClient.SendTransaction(&tx)
		if err != nil {
			log.Fatalf("Unable to send transaction: %v", err)
		}
	case "query":
		if key == nil || len(*key) == 0 {
			log.Fatalf("missing object key")
		}
		keybytes := readkey(*key)
		err = transactorClient.Query(keybytes)
		if err != nil {
			log.Fatalf("Unable to query an object: %v", err)
		}
	case "create":
		if object == nil || len(*object) <= 0 {
			log.Fatalf("missing object to create")
		}
		err = transactorClient.Create(*object)
		if err != nil {
			log.Fatalf("Unable to create a new object: %v", err)
		}
	case "delete":
		if key == nil || len(*key) == 0 {
			log.Fatalf("missing object key")
		}
		keybytes := readkey(*key)
		err = transactorClient.Delete(keybytes)
		if err != nil {
			log.Fatalf("Unable to query an object: %v", err)
		}
	default:
		log.Fatalf("Invalid command name: %s", cmd)
	}
}

func readkey(s string) []byte {
	bytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		log.Fatalf("unable to read key: %v", err)
	}
	return bytes
}
