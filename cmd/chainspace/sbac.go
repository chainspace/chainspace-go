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
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/restsrv"
	sbacclient "chainspace.io/prototype/sbac/client"
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

func cmdSBAC(args []string, usage string) {
	opts := newOpts("sbac NETWORK_NAME COMMAND [OPTIONS]", usage)
	configRoot := opts.Flags("-c", "--config-root").Label("PATH").String("path to the chainspace root directory [$HOME/.chainspace]", defaultRootDir())
	consoleLog := opts.Flags("--console-log").Label("LEVEL").String("set the minimum console log level")
	payloadPath := opts.Flags("-p", "--payload-path").Label("PATH").String("path to the payload of the transaction to send", "")
	object := opts.Flags("-o", "--object").Label("OBJECT").String("an object to create in chainspace", "")
	key := opts.Flags("-k", "--key").Label("KEY").String("a key/identifier for an object stored in chainspace, e.g [42 54 67]", "")
	networkName, cmd := getRequiredParams(opts, args)

	_, err := os.Stat(*configRoot)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Could not find the Chainspace root directory", fld.Path(*configRoot))
		}
		log.Fatal("Unable to access the Chainspace root directory", fld.Path(*configRoot), fld.Err(err))
	}

	netPath := filepath.Join(*configRoot, networkName)
	netCfg, err := config.LoadNetwork(filepath.Join(netPath, "network.yaml"))
	if err != nil {
		log.Fatal("Could not load network.yaml", fld.Err(err))
	}

	if *consoleLog != "" {
		switch *consoleLog {
		case "debug":
			log.ToConsole(log.DebugLevel)
		case "error":
			log.ToConsole(log.ErrorLevel)
		case "fatal":
			log.ToConsole(log.FatalLevel)
		case "info":
			log.ToConsole(log.InfoLevel)
		default:
			log.Fatal("Unknown --console-log level: " + *consoleLog)
		}
	}

	topology, err := network.New(networkName, netCfg)
	if err != nil {
		log.Fatal("Could not initalize network", fld.Err(err))
	}
	topology.BootstrapMDNS()

	cfg := &sbacclient.Config{
		Top:        topology,
		MaxPayload: netCfg.MaxPayload,
	}

	sbacclt := sbacclient.New(cfg)
	defer sbacclt.Close()

	switch cmd {
	case "transaction":
		if payloadPath == nil || len(*payloadPath) <= 0 {
			log.Fatal("missing required payload path")
		}
		payload, err := ioutil.ReadFile(*payloadPath)
		if err != nil {
			log.Fatal("Unable to read payload file", fld.Err(err))
		}

		tx := restsrv.Transaction{}
		err = json.Unmarshal(payload, &tx)
		if err != nil {
			log.Fatal("Invalid payload format for transaction", fld.Err(err))
		}

		ttx, _ := tx.ToSBAC()
		objects, err := sbacclt.SendTransaction(ttx, map[uint64][]byte{})
		if err != nil {
			log.Fatal("unable to send transaction", fld.Err(err))
		}
		data := []restsrv.Object{}
		for _, v := range objects {
			v := v
			o := restsrv.Object{
				VersionID: base64.StdEncoding.EncodeToString(v.VersionID),
				Value:     base64.StdEncoding.EncodeToString(v.Value),
				Status:    v.Status.String(),
			}
			data = append(data, o)
		}
		b, err := json.Marshal(data)
		if err != nil {
			log.Fatal("unable to marshal result", fld.Err(err))
		}
		fmt.Printf("%v\n", string(b))
	case "query":
		if key == nil || len(*key) == 0 {
			log.Fatal("missing object key")
		}
		keybytes := readkey(*key)
		objs, err := sbacclt.Query(keybytes)
		if err != nil {
			log.Fatal("unable to query object", fld.Err(err))
		}
		obj, err := restsrv.BuildObjectResponse(objs)
		if err != nil {
			log.Fatal("error building result", fld.Err(err))
		}
		b, err := json.Marshal(obj)
		if err != nil {
			log.Fatal("unable to marshal result", fld.Err(err))
		}
		fmt.Printf("%v\n", string(b))
	case "create":
		if key == nil || len(*object) == 0 {
			log.Fatal("missing object key")
		}
		objbytes := readkey(*object)
		ids, err := sbacclt.Create(objbytes)
		if err != nil {
			log.Fatal("unable to query object", fld.Err(err))
		}
		for _, v := range ids {
			if string(v) != string(ids[0]) {
				log.Fatal("error building result", fld.Err(errors.New("inconsistent data")))
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
			log.Fatal("unable to marshal result", fld.Err(err))
		}
		fmt.Printf("%v\n", string(b))
	case "delete":
		if key == nil || len(*key) == 0 {
			log.Fatal("missing object key")
		}
		keybytes := readkey(*key)
		objs, err := sbacclt.Delete(keybytes)
		if err != nil {
			log.Fatal("unable to delete object", fld.Err(err))
		}
		obj, err := restsrv.BuildObjectResponse(objs)
		if err != nil {
			log.Fatal("error building result", fld.Err(err))
		}
		b, err := json.Marshal(obj)
		if err != nil {
			log.Fatal("unable to marshal result", fld.Err(err))
		}
		fmt.Printf("%v\n", string(b))
	default:
		log.Fatal("invalid/unknown command", fld.SBACCmd(cmd))
	}

}

func readkey(s string) []byte {
	bytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		log.Fatal("unable to read key", fld.Err(err))
	}
	return bytes
}
