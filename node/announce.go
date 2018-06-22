package node

import (
	"fmt"
	"strings"

	"github.com/grandcat/zeroconf"
)

// TODO(tav): Ensure that server.Shutdown is called by exit handlers and use
// hostIP instead of defaulting to everything.
func announceMDNS(networkID string, nodeID uint64, hostIP string, port int) error {
	instance := fmt.Sprintf("_%d", nodeID)
	service := fmt.Sprintf("_%s._chainspace", strings.ToLower(networkID))
	_, err := zeroconf.Register(instance, service, "local.", port, nil, nil)
	return err
}
