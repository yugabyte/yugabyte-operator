package controllers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	yugabytecomv1alpha1 "github.com/yugabyte/yugabyte-operator/api/v1alpha1"
)

type certPair struct {
	cert, key string
}

func parseCertAndKey(b64encodedCert, b64encodedKey string) (*x509.Certificate, *rsa.PrivateKey, error) {
	b64decodedCert, err := base64.StdEncoding.DecodeString(b64encodedCert)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to decode Root CA Cert")
	}

	b64decodedKey, err := base64.StdEncoding.DecodeString(b64encodedKey)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to decode Root Key")
	}

	pemDecodedCert, _ := pem.Decode(b64decodedCert)

	if pemDecodedCert == nil {
		return nil, nil, fmt.Errorf("Unable to decode Root CA Cert")
	}

	parsedCert, err := x509.ParseCertificate(pemDecodedCert.Bytes)

	if err != nil {
		return nil, nil, fmt.Errorf("Unable to parse Root CA Cert. Error: %s", err)
	}

	pemDecodedKey, _ := pem.Decode(b64decodedKey)

	if pemDecodedKey == nil {
		return nil, nil, fmt.Errorf("Unable to decode Root Key")
	}

	parsedKey, err := x509.ParsePKCS1PrivateKey(pemDecodedKey.Bytes)

	if err != nil {
		return nil, nil, fmt.Errorf("Unable to parse Root Key. Error: %s", err)
	}

	return parsedCert, parsedKey, nil
}

func generateSignedCerts(cn string, alternateDNS []string, daysValid int, rootCA *yugabytecomv1alpha1.YBRootCASpec) (*certPair, error) {
	rootCert, rootKey, err := parseCertAndKey(rootCA.Cert, rootCA.Key)

	if err != nil {
		return nil, err
	}

	certTemplate, err := getCertTemplate(cn, alternateDNS, daysValid)

	if err != nil {
		return nil, err
	}

	serverKeys, err := rsa.GenerateKey(rand.Reader, 2048)

	if err != nil {
		return nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, rootCert, serverKeys.Public(), rootKey)

	if err != nil {
		return nil, fmt.Errorf("Error creating server certificates. Error: %s", err)
	}

	encCertBuffer := bytes.Buffer{}
	if err := pem.Encode(&encCertBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return nil, fmt.Errorf("Error encoding server certificate: %s", err)
	}

	encKeyBuffer := bytes.Buffer{}
	if err := pem.Encode(&encKeyBuffer, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKeys)}); err != nil {
		return nil, fmt.Errorf("Error encoding server keys: %s", err)
	}

	return &certPair{cert: string(encCertBuffer.Bytes()), key: string(encKeyBuffer.Bytes())}, nil
}

func getCertTemplate(cn string, alternateDNS []string, daysValid int) (*x509.Certificate, error) {
	srNoUpperLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	srNo, err := rand.Int(rand.Reader, srNoUpperLimit)

	if err != nil {
		return nil, err
	}

	return &x509.Certificate{
		SerialNumber: srNo,
		Subject: pkix.Name{
			CommonName: cn,
		},
		IPAddresses: []net.IP{},
		DNSNames:    alternateDNS,
		NotBefore:   time.Now().AddDate(0, 0, -1),
		NotAfter:    time.Now().Add(time.Hour * 24 * time.Duration(daysValid)),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
	}, nil
}
