// Copyright 2019 GRAIL, Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.
package tls

import (
	"crypto/rsa"
	"crypto/x509"
	"net"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/grailbio/infra"
	"github.com/grailbio/testutil"
)

type issuer interface {
	Issue(cn string, ttl time.Duration, ips []net.IP, dnss []string) ([]byte, *rsa.PrivateKey, error)
	Certificate() *x509.Certificate
}

func TestAuthority(t *testing.T) {
	dir, cleanup := testutil.TempDir(t, "", "")
	defer cleanup()
	path := filepath.Join(dir, "authority")
	schema := infra.Schema{"tls": new(issuer)}
	config, err := schema.Make(infra.Keys{
		"tls": "tls,file=" + path,
	})
	if err != nil {
		t.Fatal(err)
	}
	var issuer issuer
	config.Must(&issuer)
	testIssuer(t, issuer)
}

func TestAuthorityMarshal(t *testing.T) {
	dir, cleanup := testutil.TempDir(t, "", "")
	defer cleanup()
	path := filepath.Join(dir, "authority")
	schema := infra.Schema{"tls": new(issuer)}
	config, err := schema.Make(infra.Keys{
		"tls": "tls,file=" + path,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := config.Marshal(true)
	if err != nil {
		t.Fatal(err)
	}
	// Now make sure the restored authority still works.
	config, err = schema.Unmarshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var issuer issuer
	config.Must(&issuer)
	testIssuer(t, issuer)
}

func testIssuer(t *testing.T, issuer issuer) {
	t.Helper()
	now := time.Now()
	ips := []net.IP{net.IPv4(1, 2, 3, 4)}
	dnses := []string{"test.grail.com"}
	certBytes, priv, err := issuer.Issue("test", 10*time.Minute, ips, dnses)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		t.Fatal(err)
	}
	opts := x509.VerifyOptions{}
	opts.Roots = x509.NewCertPool()
	opts.Roots.AddCert(issuer.Certificate())
	if _, err := cert.Verify(opts); err != nil {
		t.Fatal(err)
	}
	if err := priv.Validate(); err != nil {
		t.Fatal(err)
	}
	if got, want := priv.Public(), cert.PublicKey; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := cert.Subject.CommonName, "test"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := cert.NotBefore, now.Add(-driftMargin); want.Before(got) {
		t.Errorf("wanted %s <= %s", got, want)
	}
	if got, want := cert.NotAfter.Sub(cert.NotBefore), 10*time.Minute+driftMargin; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if cert.IsCA {
		t.Error("cert is CA")
	}
	if got, want := cert.IPAddresses, ips; !ipsEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := cert.DNSNames, dnses; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func ipsEqual(x, y []net.IP) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if !x[i].Equal(y[i]) {
			return false
		}
	}
	return true
}
