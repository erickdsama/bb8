package link

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Names each side must present. A cert from the CA with another name is refused.
const (
	BodyName = "bb8-body"
	HeadName = "bb8-head"
)

// Pair creates a private CA plus the body and head certs in dir (Ed25519, 10 years).
// extraHosts are added to the body cert SANs (hostnames or IPs); the head verifies
// the body by name BodyName, so they are optional.
func Pair(dir string, extraHosts []string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "ca.crt")); err == nil {
		return fmt.Errorf("%s already has a CA; delete it to pair again", dir)
	}
	now := time.Now().Add(-time.Hour)
	caPub, caKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	caTmpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: "BB-8 private CA"},
		NotBefore: now, NotAfter: now.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, caPub, caKey)
	if err != nil {
		return err
	}
	ca, _ := x509.ParseCertificate(caDER)
	if err := writePEM(filepath.Join(dir, "ca.crt"), "CERTIFICATE", caDER, 0o644); err != nil {
		return err
	}
	if err := writeKey(filepath.Join(dir, "ca.key"), caKey); err != nil {
		return err
	}
	issue := func(name string, usage x509.ExtKeyUsage, hosts []string) error {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		t := &x509.Certificate{
			SerialNumber: serial(), Subject: pkix.Name{CommonName: name},
			NotBefore: now, NotAfter: now.AddDate(10, 0, 0),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
			DNSNames: []string{name},
		}
		for _, h := range hosts {
			if ip := net.ParseIP(h); ip != nil {
				t.IPAddresses = append(t.IPAddresses, ip)
			} else {
				t.DNSNames = append(t.DNSNames, h)
			}
		}
		der, err := x509.CreateCertificate(rand.Reader, t, ca, pub, caKey)
		if err != nil {
			return err
		}
		if err := writePEM(filepath.Join(dir, name+".crt"), "CERTIFICATE", der, 0o644); err != nil {
			return err
		}
		return writeKey(filepath.Join(dir, name+".key"), key)
	}
	if err := issue(BodyName, x509.ExtKeyUsageServerAuth, extraHosts); err != nil {
		return err
	}
	return issue(HeadName, x509.ExtKeyUsageClientAuth, nil)
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 126))
	return n
}

func writePEM(path, typ string, der []byte, mode os.FileMode) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), mode)
}

func writeKey(path string, key ed25519.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return writePEM(path, "PRIVATE KEY", der, 0o600)
}

func load(dir, name string) (tls.Certificate, *x509.CertPool, error) {
	cert, err := tls.LoadX509KeyPair(filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key"))
	if err != nil {
		return cert, nil, err
	}
	caPEM, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	if err != nil {
		return cert, nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return cert, nil, errors.New("bad ca.crt")
	}
	return cert, pool, nil
}

func expectPeer(name string) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 || cs.PeerCertificates[0].Subject.CommonName != name {
			return fmt.Errorf("peer is not %s", name)
		}
		return nil
	}
}

// ServerTLS is the body side: requires a head cert signed by the BB-8 CA.
func ServerTLS(dir string) (*tls.Config, error) {
	cert, pool, err := load(dir, BodyName)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert},
		ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert,
		VerifyConnection: expectPeer(HeadName),
	}, nil
}

// ClientTLS is the head side: trusts only the BB-8 CA and the name bb8-body.
func ClientTLS(dir string) (*tls.Config, error) {
	cert, pool, err := load(dir, HeadName)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert},
		RootCAs: pool, ServerName: BodyName, VerifyConnection: expectPeer(BodyName),
	}, nil
}
