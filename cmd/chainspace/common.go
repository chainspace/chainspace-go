package main

import (
	"os"

	"github.com/tav/golly/fsutil"
	"github.com/tav/golly/log"
	"github.com/tav/golly/optparse"
	"gopkg.in/yaml.v2"
)

const (
	dirPerms = 0700
)

func createUnlessExists(path string) {
	if exists, _ := fsutil.Exists(path); exists {
		log.Fatalf("A directory already exists at: %s", path)
	}
	if err := os.Mkdir(path, dirPerms); err != nil {
		log.Fatal(err)
	}
}

func ensureRootDir() (string, error) {
	path := os.ExpandEnv("$HOME/.chainspace")
	if exists, err := fsutil.Exists(path); exists {
		return path, err
	}
	return path, os.Mkdir(path, dirPerms)
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
