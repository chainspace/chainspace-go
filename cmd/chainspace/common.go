package main

import (
	"os"
	"strconv"

	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"github.com/tav/golly/fsutil"
	"github.com/tav/golly/optparse"
	"gopkg.in/yaml.v2"
)

const (
	dirPerms = 0700
)

func createUnlessExists(path string) {
	if exists, _ := fsutil.Exists(path); exists {
		log.Fatal("Directory already exists", fld.Path(path))
	}
	if err := os.Mkdir(path, dirPerms); err != nil {
		log.Fatal("Could not create directory", fld.Path(path), fld.Err(err))
	}
}

func defaultRootDir() string {
	return os.ExpandEnv("$HOME/.chainspace")
}

func ensureDir(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Mkdir(path, dirPerms)
		}
		return err
	}
	return nil
}

func getNetworkNameAndNodeID(opts *optparse.Parser, args []string) (string, uint64) {
	params := opts.Parse(args)
	if len(params) != 2 {
		opts.PrintUsage()
		os.Exit(1)
	}
	networkName := params[0]
	if networkName == "" {
		log.Fatal("Network name cannot be empty")
	}
	nodeID, err := strconv.ParseUint(params[1], 10, 64)
	if err != nil {
		log.Fatal("Could not parse Node ID", fld.Err(err))
	}
	return networkName, nodeID
}

func newOpts(command string, usage string) *optparse.Parser {
	return optparse.New("Usage: chainspace " + command + "\n\n  " + usage + "\n")
}

func writeYAML(path string, v interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	return enc.Encode(v)
}
