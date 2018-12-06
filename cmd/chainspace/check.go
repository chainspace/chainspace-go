package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"chainspace.io/prototype/checker"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	sbacapi "chainspace.io/prototype/sbac/api"

	"github.com/tav/golly/optparse"
)

func getNetworkName(opts *optparse.Parser, args []string) string {
	params := opts.Parse(args)
	if len(params) != 1 {
		opts.PrintUsage()
		os.Exit(1)
	}
	networkName := params[0]
	if networkName == "" {
		log.Fatal("Network name cannot be empty")
	}
	return networkName
}

func cmdCheck(args []string, usage string) {
	opts := newOpts("check NETWORK_NAME [OPTIONS]", usage)
	typecheckOnly := opts.Flags("--tycheck-only").Label("BOOL").Bool("Only run the typechecker")
	transaction := opts.Flags("--tx").Label("PATH").String("Path to the file containing the transaction to check")
	networkName := getNetworkName(opts, args)
	_ = networkName
	_ = typecheckOnly

	// read transaction
	if transaction == nil || len(*transaction) <= 0 {
		log.Fatal("missing transaction path")
	}
	b, err := ioutil.ReadFile(*transaction)
	if err != nil {
		log.Fatal("unable to read transaction", fld.Err(err))
	}
	var tx sbacapi.Transaction
	err = json.Unmarshal(b, &tx)
	if err != nil {
		log.Fatal("unable to unmarshal transction", fld.Err(err))
	}

	var result bool
	if *typecheckOnly {
		result = typeCheckOnly(&tx)
	} else {
		fullCheck(&tx, networkName)
	}
	fmt.Printf("%v\n", result)
}

func typeCheckOnly(tx *sbacapi.Transaction) bool {
	t, err := tx.ToSBAC(sbacapi.Validator{})
	if err != nil {
		log.Fatal("invalid transaction", fld.Err(err))
	}
	err = checker.TypeCheck(t)
	if err != nil {
		log.Fatal("invalid transaction", fld.Err(err))
	}
	return true
}

func fullCheck(tx *sbacapi.Transaction, networkName string) bool {
	return true
}
