package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/restsrv"
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

	topology, err := network.New(networkName, netCfg)
	if err != nil {
		log.Fatal("Could not initalize network", zap.Error(err))
	}
	topology.BootstrapMDNS()

	cfg := &transactorclient.Config{
		Top:        topology,
		MaxPayload: netCfg.MaxPayload,
	}

	transactorClient := transactorclient.New(cfg)
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

		tx := restsrv.Transaction{}
		err = json.Unmarshal(payload, &tx)
		if err != nil {
			log.Fatal("Invalid payload format for transaction", zap.Error(err))
		}

		objects, err := transactorClient.SendTransaction(tx.ToTransactor())
		if err != nil {
			log.Fatal("unable to send transaction", zap.Error(err))
		}
		data := []restsrv.Object{}
		for _, v := range objects {
			v := v
			o := restsrv.Object{
				Key:    base64.StdEncoding.EncodeToString(v.Key),
				Value:  base64.StdEncoding.EncodeToString(v.Value),
				Status: v.Status.String(),
			}
			data = append(data, o)
		}
		b, err := json.Marshal(data)
		if err != nil {
			log.Fatal("unable to marshal result", zap.Error(err))
		}
		fmt.Printf("%v\n", string(b))
	case "query":
		if key == nil || len(*key) == 0 {
			log.Fatal("missing object key")
		}
		keybytes := readkey(*key)
		objs, err := transactorClient.Query(keybytes)
		if err != nil {
			log.Fatal("unable to query object", zap.Error(err))
		}
		obj, err := restsrv.BuildObjectResponse(objs)
		if err != nil {
			log.Fatal("error building result", zap.Error(err))
		}
		b, err := json.Marshal(obj)
		if err != nil {
			log.Fatal("unable to marshal result", zap.Error(err))
		}
		fmt.Printf("%v\n", string(b))
	case "create":
		if key == nil || len(*object) == 0 {
			log.Fatal("missing object key")
		}
		objbytes := readkey(*object)
		ids, err := transactorClient.Create(objbytes)
		if err != nil {
			log.Fatal("unable to query object", zap.Error(err))
		}
		for _, v := range ids {
			if string(v) != string(ids[0]) {
				log.Fatal("error building result", zap.Error(errors.New("inconsistent data")))
				return
			}
		}
		res := struct {
			ID string `json:"id"`
		}{
			ID: base64.StdEncoding.EncodeToString(ids[0]),
		}
		b, err := json.Marshal(res)
		if err != nil {
			log.Fatal("unable to marshal result", zap.Error(err))
		}
		fmt.Printf("%v\n", string(b))
	case "delete":
		if key == nil || len(*key) == 0 {
			log.Fatal("missing object key")
		}
		keybytes := readkey(*key)
		objs, err := transactorClient.Delete(keybytes)
		if err != nil {
			log.Fatal("unable to delete object", zap.Error(err))
		}
		obj, err := restsrv.BuildObjectResponse(objs)
		if err != nil {
			log.Fatal("error building result", zap.Error(err))
		}
		b, err := json.Marshal(obj)
		if err != nil {
			log.Fatal("unable to marshal result", zap.Error(err))
		}
		fmt.Printf("%v\n", string(b))
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
