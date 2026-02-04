package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func main() {
	var service, namespace, outDir string
	flag.StringVar(&service, "service", "", "Service name")
	flag.StringVar(&namespace, "namespace", "", "Namespace")
	flag.StringVar(&outDir, "out-dir", ".", "Output directory")
	flag.Parse()

	if service == "" || namespace == "" {
		fmt.Fprintf(os.Stderr, "service and namespace required\n")
		os.Exit(1)
	}

	// CA Config
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2026),
		Subject: pkix.Name{
			CommonName: "Admission Webhook CA",
		},
		NotBefore:             time.Now().Add(-24 * time.Hour), // Backdate 1 day
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		panic(err)
	}

	// Server Config
	dnsNames := []string{
		service,
		fmt.Sprintf("%s.%s", service, namespace),
		fmt.Sprintf("%s.%s.svc", service, namespace),
	}

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2026),
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s.%s.svc", service, namespace),
		},
		DNSNames:    dnsNames,
		NotBefore:   time.Now().Add(-24 * time.Hour), // Backdate 1 day
		NotAfter:    time.Now().Add(10 * 365 * 24 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &serverPrivKey.PublicKey, caPrivKey)
	if err != nil {
		panic(err)
	}

	// Write files
	writePem(filepath.Join(outDir, "ca.crt"), "CERTIFICATE", caBytes)
	writePem(filepath.Join(outDir, "ca.key"), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caPrivKey))
	writePem(filepath.Join(outDir, "tls.crt"), "CERTIFICATE", certBytes)
	writePem(filepath.Join(outDir, "tls.key"), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverPrivKey))
}

func writePem(path, typeName string, bytes []byte) {
	out, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	pem.Encode(out, &pem.Block{
		Type:  typeName,
		Bytes: bytes,
	})
}
