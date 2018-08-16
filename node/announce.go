package node

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/x509certs"

	"github.com/grandcat/zeroconf"
)

const (
	contactsSetPath = "contacts.set"
)

var (
	client *http.Client
	pool   *x509.CertPool
)

// TODO(tav): Ensure that server.Shutdown is called by exit handlers and use
// hostIP instead of defaulting to everything.
func announceMDNS(networkID string, nodeID uint64, port int) error {
	instance := fmt.Sprintf("_%d", nodeID)
	service := fmt.Sprintf("_%s._chainspace", strings.ToLower(networkID))
	_, err := zeroconf.Register(instance, service, "local.", port, nil, nil)
	return err
}

func announceRegistry(registries []config.Registry, networkID string, nodeID uint64, port int) error {
	for _, v := range registries {
		u, _ := url.Parse(v.Host)
		u.Path = path.Join(u.Path, contactsSetPath)
		s := fmt.Sprintf(`{"auth": {"network_id": "%v", "token": "%v"}, "config": {"node_id": %v, "port": %v}}`, networkID, v.Token, nodeID, port)
		payload := bytes.NewBufferString(s)
		// log.Error("announce to registry", log.String("lol", s))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequest(http.MethodPost, u.String(), payload)
		req = req.WithContext(ctx)
		req.Header.Add("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error calling registry: %v", err)
		}
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("registry answered with unexpected http status: %v", resp.StatusCode)
		}
	}
	return nil
}

func init() {
	pool = x509.NewCertPool()
	pool.AppendCertsFromPEM(x509certs.PemCerts)
	client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
}
