// Package transport provides support for transport-layer certificates.
package transport // import "chainspace.io/prototype/crypto/transport"

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// The various types of transport-layer certificates that are currently
// supported.
const (
	ECDSA CertType = iota + 1
)

// CertType represents a type of transport layer certificate.
type CertType uint8

func (c CertType) String() string {
	switch c {
	case ECDSA:
		return "ecdsa"
	}
	return ""
}

// Cert represents the public and private components of a PEM-encoded X.509
// certificate.
type Cert struct {
	Private string
	Public  string
	Type    CertType
}

// GenCert creates a new cert for the given cert type. It uses crypto/rand's
// Reader behind the scenes as the source of randomness.
func GenCert(t CertType) (*Cert, error) {
	switch t {
	case ECDSA:
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		der, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, err
		}
		tmpl := &x509.Certificate{
			NotAfter:     time.Now().Add(time.Hour * 24 * 365 * 100), // 100 years
			NotBefore:    time.Now(),
			SerialNumber: big.NewInt(1),
		}
		cert, err := x509.CreateCertificate(
			rand.Reader, tmpl, tmpl, &key.PublicKey, key,
		)
		if err != nil {
			return nil, err
		}
		c := &Cert{
			Type: ECDSA,
		}
		buf := &bytes.Buffer{}
		block := &pem.Block{
			Bytes: der,
			Type:  "EC PRIVATE KEY",
		}
		pem.Encode(buf, block)
		c.Private = buf.String()
		buf.Reset()
		block = &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		}
		pem.Encode(buf, block)
		c.Public = buf.String()
		return c, nil
	default:
		return nil, fmt.Errorf("transport: unknown cert type: %s", t)
	}
}
