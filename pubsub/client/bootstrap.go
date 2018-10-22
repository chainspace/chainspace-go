package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"github.com/grandcat/zeroconf"
)

type mdnscallback func(nodeID uint64, addr string)

func bootstrapMDNS(ctx context.Context, network string, cb mdnscallback) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}
	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for {
			select {
			case entry, ok := <-entries:
				if !ok {
					return
				}
				instance := entry.ServiceRecord.Instance
				if !strings.HasPrefix(instance, "_") {
					continue
				}
				nodeID, err := strconv.ParseUint(instance[1:], 10, 64)
				if err != nil {
					continue
				}
				if len(entry.AddrIPv4) > 0 && entry.Port > 0 {
					addr := fmt.Sprintf("%s:%d", entry.AddrIPv4[0].String(), entry.Port)
					cb(nodeID, addr)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	service := fmt.Sprintf("_%s_pubsub._chainspace", strings.ToLower(network))
	return resolver.Browse(ctx, service, "local.", entries)
}

// BootstrapMDNS will try to auto-discover the addresses of initial nodes using
// multicast DNS.
func BootstrapMDNS(network string, cb mdnscallback) {
	log.Debug("Bootstrapping network via mDNS", fld.NetworkName(network))
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			if err := bootstrapMDNS(ctx, network, cb); err != nil {
				log.Error("Unable to start bootstrapping mDNS", log.Err(err))
			}
			select {
			case <-ctx.Done():
				cancel()
			}
		}
	}()
}
