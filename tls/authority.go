// Copyright 2019 GRAIL, Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package tls implements TLS infrastructure providers.
package tls

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	cryptotls "crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/grailbio/infra"
)

func init() { infra.Register("tls", new(Authority)) }

var (
	certDuration = 27 * 7 * 24 * time.Hour

	// TODO(marius): make this configurable.
	caCertExpiry = time.Date(3000, time.January, 0, 0, 0, 0, 0, time.UTC)

	// TODO(marius): make this configurable.
	driftMargin = time.Minute
)

// Authority is an infrastructure provider that implements a TLS
// authority, capable of issuing TLS certificates. Its implementation
// requires that a file be specified (through the flag parameter
// file) where the authority is stored. An Authority's instance may
// also be marshaled, in which case the certificate material for the
// authority is inlined directly in the instance configuration.
type Authority struct {
	path string
	key  *rsa.PrivateKey
	cert *x509.Certificate

	// The CA certificate and key are stored in PEM-encoded bytes
	// as most of the Go APIs operate directly on these.
	certPEM, keyPEM []byte

	pemBlock *string
}

// Help implements infra.Provider
func (Authority) Help() string {
	return "configure a HTTPS CA from the provided PEM-encoded signing certificate"
}

// Flags implements infra.Provider.
func (ca *Authority) Flags(flags *flag.FlagSet) {
	flags.StringVar(&ca.path, "file", "", "path of file where the certificate authority is stored")
}

// Init implements infra.Provider. It initializes the authority from
// either the provided file or the serialized instance configuration.
func (ca *Authority) Init() error {
	if ca.path == "" && (ca.pemBlock == nil || len(*ca.pemBlock) == 0) {
		return errors.New("tls.Authority: no authority file specified")
	}

	if ca.path != "" && (ca.pemBlock == nil || len(*ca.pemBlock) == 0) {
		// As an extra precaution, we always exercise the read path, so if
		// the CA PEM is missing, we generate it, and then read it back.
		if _, err := os.Stat(ca.path); os.IsNotExist(err) {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				return err
			}
			template := x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					// TODO(marius): make this configurable.
					CommonName: "infra",
				},
				NotBefore:             time.Now(),
				NotAfter:              caCertExpiry,
				KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
				BasicConstraintsValid: true,
				IsCA:                  true,
			}
			cert, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
			if err != nil {
				return err
			}
			f, err := os.Create(ca.path)
			if err != nil {
				return err
			}
			// Save it also.
			if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
				f.Close()
				return err
			}
			if err := pem.Encode(f, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	if ca.pemBlock == nil {
		ca.pemBlock = new(string)
	}
	var pemBlock []byte
	if len(*ca.pemBlock) == 0 {
		var err error
		pemBlock, err = ioutil.ReadFile(ca.path)
		if err != nil {
			return err
		}
		*ca.pemBlock = string(pemBlock)
	} else {
		pemBlock = []byte(*ca.pemBlock)
	}
	var certBlock, keyBlock []byte
	for {
		var derBlock *pem.Block
		derBlock, pemBlock = pem.Decode(pemBlock)
		if derBlock == nil {
			break
		}
		switch derBlock.Type {
		case "CERTIFICATE":
			certBlock = derBlock.Bytes
		case "RSA PRIVATE KEY":
			keyBlock = derBlock.Bytes
		}
	}

	if certBlock == nil || keyBlock == nil {
		return errors.New("tls.Authority: incomplete certificate")
	}
	var err error
	ca.cert, err = x509.ParseCertificate(certBlock)
	if err != nil {
		return err
	}
	ca.key, err = x509.ParsePKCS1PrivateKey(keyBlock)
	if err != nil {
		return err
	}
	ca.certPEM, err = encodePEM(&pem.Block{Type: "CERTIFICATE", Bytes: certBlock})
	if err != nil {
		return err
	}
	ca.keyPEM, err = encodePEM(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(ca.key)})
	if err != nil {
		return err
	}
	return nil
}

// Certificate returns the authority's public certificate, which can be used
// to verify certificates issued by the same.
func (ca *Authority) Certificate() *x509.Certificate { return ca.cert }

// InstanceConfig implements infra.Provider, allowing for the authority's
// certificate material to be marshaled inline.
func (ca *Authority) InstanceConfig() interface{} {
	if ca.pemBlock == nil {
		ca.pemBlock = new(string)
	}
	return ca.pemBlock
}

// Issue issues a new certificate out of this CA with the provided
// common name, TTL, IPs, and DNS names.
func (ca *Authority) Issue(cn string, ttl time.Duration, ips []net.IP, dnss []string) ([]byte, *rsa.PrivateKey, error) {
	maxSerial := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, maxSerial)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().Add(-driftMargin)
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             now,
		NotAfter:              now.Add(driftMargin + ttl),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	template.IPAddresses = append(template.IPAddresses, ips...)
	template.DNSNames = append(template.DNSNames, dnss...)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.CreateCertificate(rand.Reader, &template, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

// HTTPS returns a tls configs based on newly issued TLS certificates from this CA.
func (ca *Authority) HTTPS() (client, server *cryptotls.Config, err error) {
	cert, key, err := ca.Issue("infra", certDuration, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca.certPEM)

	// Load the newly created certificate.
	certPEM, err := encodePEM(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := encodePEM(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err != nil {
		return nil, nil, err
	}
	tlscert, err := cryptotls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, err
	}
	clientConfig := &cryptotls.Config{
		RootCAs:            pool,
		InsecureSkipVerify: true,
		Certificates:       []cryptotls.Certificate{tlscert},
	}
	serverConfig := &cryptotls.Config{
		ClientCAs:    pool,
		Certificates: []cryptotls.Certificate{tlscert},
	}
	return clientConfig, serverConfig, nil
}

func encodePEM(block *pem.Block) ([]byte, error) {
	var w bytes.Buffer
	if err := pem.Encode(&w, block); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}
