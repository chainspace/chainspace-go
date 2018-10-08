package main

import (
	"os"
	"path/filepath"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/contracts"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
)

func cmdContracts(args []string, usage string) {
	opts := newOpts("contracts NETWORK_NAME COMMAND [OPTIONS]", usage)
	configRoot := opts.Flags("--config-root").Label("PATH").String("Path to the chainspace root directory [~/.chainspace]", defaultRootDir())
	networkName, cmd := getRequiredParams(opts, args)

	_, err := os.Stat(*configRoot)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Could not find the Chainspace root directory", fld.Path(*configRoot))
		}
		log.Fatal("Unable to access the Chainspace root directory", fld.Path(*configRoot), fld.Err(err))
	}

	netPath := filepath.Join(*configRoot, networkName)
	cfg, err := config.LoadContracts(filepath.Join(netPath, "contracts.yaml"))
	if err != nil {
		log.Fatal("Could not load contracts.yaml", fld.Err(err))
	}

	cts, err := contracts.New(cfg)
	if err != nil {
		log.Fatal("unable to instantiate contacts", fld.Err(err))
	}

	switch cmd {
	case "create":
		err = cts.Start()
		if err != nil {
			log.Fatal("unable to start contracts", fld.Err(err))
		}
	case "destroy":
		err = cts.Stop()
		if err != nil {
			log.Fatal("uable to stop contracts", fld.Err(err))
		}
	default:
		log.Fatal("invalid/unknown command", log.String("cmd", cmd))
	}
}
