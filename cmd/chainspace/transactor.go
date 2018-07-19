package main

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/transactor/client"

	"github.com/tav/golly/optparse"
	"go.uber.org/zap"
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
			log.Fatal("Could not find the Chainspace root directory", zap.String("dir", *configRoot))
		}
		log.Fatal("Unable to access the Chainspace root directory", zap.String("dir", *configRoot), zap.Error(err))
	}

	netPath := filepath.Join(*configRoot, networkName)
	netCfg, err := config.LoadNetwork(filepath.Join(netPath, "network.yaml"))
	if err != nil {
		log.Fatal("Could not load network.yaml", zap.Error(err))
	}

	cfg := &transactorclient.Config{
		NetworkName:   networkName,
		NetworkConfig: *netCfg,
	}

	transactorClient, err := transactorclient.New(cfg)
	if err != nil {
		log.Fatal("Unable to create transactor.Client", zap.Error(err))
	}
	defer transactorClient.Close()

	switch cmd {
	case "transaction":
		if payloadPath == nil || len(*payloadPath) <= 0 {
			log.Fatal("missing required payload path")
		}
		payload, err := ioutil.ReadFile(*payloadPath)
		if err != nil {
			log.Fatal("Unable to read payload file", zap.Error(err))
		}

		tx := transactorclient.ClientTransaction{}
		err = json.Unmarshal(payload, &tx)
		if err != nil {
			log.Fatal("Invalid payload format for transaction", zap.Error(err))
		}
		err = transactorClient.SendTransaction(&tx)
		if err != nil {
			log.Fatal("Unable to send transaction", zap.Error(err))
		}
	case "query":
		if key == nil || len(*key) == 0 {
			log.Fatal("missing object key")
		}
		keybytes := readkey(*key)
		err = transactorClient.Query(keybytes)
		if err != nil {
			log.Fatal("Unable to query an object", zap.Error(err))
		}
	case "create":
		if object == nil || len(*object) <= 0 {
			log.Fatal("missing object to create")
		}
		err = transactorClient.Create(*object)
		if err != nil {
			log.Fatal("Unable to create a new object", zap.Error(err))
		}
	case "delete":
		if key == nil || len(*key) == 0 {
			log.Fatal("missing object key")
		}
		keybytes := readkey(*key)
		err = transactorClient.Delete(keybytes)
		if err != nil {
			log.Fatal("Unable to query an object", zap.Error(err))
		}
	default:
		log.Fatal("invalid/unknown command", zap.String("cmd", cmd))
	}
}

func readkey(s string) []byte {
	bytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		log.Fatal("unable to read key", zap.Error(err))
	}
	return bytes
}
