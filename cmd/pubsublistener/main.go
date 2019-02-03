package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"chainspace.io/chainspace-go/pubsub/client"
)

var (
	addrs       string
	port        int
	mdns        bool
	nodeCount   int
	gcp         bool
	networkName string
)

func init() {
	flag.StringVar(&addrs, "addrs", "", "list of addresses to listen to")
	flag.IntVar(&port, "port", 7000, "port to bind to")
	flag.BoolVar(&gcp, "gcp", false, "are the node deployed on gcp")
	flag.IntVar(&nodeCount, "node-count", 4, "number of node to connect to")
	flag.BoolVar(&mdns, "mdns", false, "use mdns")
	flag.StringVar(&networkName, "network-name", "", "name of the network to use")
}

func splitAddresses(s string) map[uint64]string {
	split := strings.Split(s, ",")
	out := map[uint64]string{}
	for _, v := range split {
		sp := strings.Split(v, "=")
		i, err := strconv.Atoi(sp[0])
		if err != nil {
			fmt.Printf("error, unable to strconv nodeid: %v", err)
			os.Exit(1)
		}
		out[uint64(i)] = strings.TrimSpace(sp[1])
	}
	return out
}

func pubsubCallback(nodeID uint64, versionID string, success bool, labels []string) {
	fmt.Printf("node-%v => success=%v labels=%v versionID=%v\n", nodeID, success, labels, versionID)
}

func main() {
	flag.Parse()
	nodesAddr := map[uint64]string{}
	if !mdns {
		if len(addrs) > 0 {
			nodesAddr = splitAddresses(addrs)
		} else if port > 0 && !gcp {
			for i := 1; i <= nodeCount; i += 1 {
				nodesAddr[uint64(i)] = fmt.Sprintf("0.0.0.0:%v", port+i)
			}
		} else if port > 0 && gcp {
			// get from gcp
		} else {
			fmt.Printf("error: missing nodes configuration\n")
		}
		fmt.Printf("subscribing to:\n")
		for k, v := range nodesAddr {
			fmt.Printf("  node-%v => %v\n", k, v)
		}

	}

	ctx, cancel := context.WithCancel(context.Background())
	cfg := client.Config{
		NetworkName: networkName,
		NodeAddrs:   nodesAddr,
		CB:          pubsubCallback,
		Ctx:         ctx,
	}

	clt := client.New(&cfg)
	_ = clt
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	<-sigc
	cancel()
	fmt.Printf("exiting...\n")
}
