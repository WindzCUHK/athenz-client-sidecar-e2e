/*
Copyright (C)  2018 Yahoo Japan Corporation Athenz team.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"

	"github.com/AthenZ/athenz-client-sidecar/v2/config"
	"github.com/pkg/errors"
)

var (
	// ErrTLSCertOrKeyNotFound represents an error that TLS cert or key is not found on the specified file path.
	ErrTLSCertOrKeyNotFound = errors.New("Cert/Key path not found")
)

// NewTLSConfig returns a *tls.Config struct or error.
// It reads TLS configuration and initializes *tls.Config struct.
// It initializes TLS configuration, for example the CA certificate and key to start TLS server.
// Server and CA Certificate, and private key will be read from files from file paths defined in environment variables.
func NewTLSConfig(cfg config.TLS) (*tls.Config, error) {
	t := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.CurveP521,
			tls.CurveP384,
			tls.CurveP256,
			tls.X25519,
		},
		SessionTicketsDisabled: true,
		ClientAuth:             tls.NoClientCert,
	}

	cert := config.GetActualValue(cfg.CertPath)
	key := config.GetActualValue(cfg.KeyPath)
	ca := config.GetActualValue(cfg.CAPath)

	if cert == "" || key == "" {
		return nil, ErrTLSCertOrKeyNotFound
	}

	crt, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	t.Certificates = make([]tls.Certificate, 1)
	t.Certificates[0] = crt

	if ca != "" {
		pool, err := NewX509CertPool(ca)
		if err != nil {
			return nil, err
		}
		t.ClientCAs = pool
		t.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return t, nil
}

// NewX509CertPool returns *x509.CertPool struct or error.
// The CertPool will read the certificate from the path, and append the content to the system certificate pool.
func NewX509CertPool(path string) (*x509.CertPool, error) {
	var pool *x509.CertPool
	c, err := ioutil.ReadFile(path)
	if err == nil && c != nil {
		pool, err = x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(c) {
			err = errors.New("Certification Failed")
		}
	}
	return pool, err
}

// NewTLSClientConfig returns a client *tls.Config struct or error.
func NewTLSClientConfig(rootCAs *x509.CertPool, certPath, certKeyPath string) (*tls.Config, error) {
	t := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}

	if certPath != "" {
		cp := config.GetActualValue(certPath)
		_, err := os.Stat(cp)
		if os.IsNotExist(err) {
			return nil, errors.New("client certificate not found")
		}
		ckp := config.GetActualValue(certKeyPath)
		_, err = os.Stat(ckp)
		if os.IsNotExist(err) {
			return nil, errors.New("client certificate key not found")
		}

		cert, err := tls.LoadX509KeyPair(cp, ckp)
		if err != nil {
			return nil, err
		}
		t.Certificates = []tls.Certificate{cert}
	}

	return t, nil
}
