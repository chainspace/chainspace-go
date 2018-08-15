package node

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"chainspace.io/prototype/log"

	"github.com/grandcat/zeroconf"
)

// TODO(tav): Ensure that server.Shutdown is called by exit handlers and use
// hostIP instead of defaulting to everything.
func announceMDNS(networkID string, nodeID uint64, port int) error {
	instance := fmt.Sprintf("_%d", nodeID)
	service := fmt.Sprintf("_%s._chainspace", strings.ToLower(networkID))
	_, err := zeroconf.Register(instance, service, "local.", port, nil, nil)
	return err
}

func announceRegistry(endpoint string, networkID, token string, nodeID uint64, port int) error {
	url, _ := url.Parse(endpoint)
	s := fmt.Sprintf(`{"auth": {"network_id": "%v", "token": "%v"}, "config": {"node_id": %v, "port": %v}}`, networkID, token, nodeID, port)
	payload := bytes.NewBufferString(s)
	log.Error("announce to registry", log.String("lol", s))
	client := http.Client{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequest(http.MethodPost, url.String(), payload)
	req = req.WithContext(ctx)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error calling registry: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("registry answered with unexpected http status: %v", resp.StatusCode)
	}

	return nil
}
