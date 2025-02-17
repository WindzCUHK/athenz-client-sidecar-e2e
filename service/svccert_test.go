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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/AthenZ/athenz-client-sidecar/v2/config"
	"github.com/AthenZ/athenz/clients/go/zts"
	"github.com/kpango/fastime"
	"github.com/kpango/glg"
	"github.com/kpango/ntokend"
)

func init() {
	glg.Get().SetMode(glg.NONE)
}

func TestIsValidDomain(t *testing.T) {
	cases := []struct {
		domain string
		expect bool
	}{
		{
			domain: "test.domain",
			expect: true,
		},
		{
			domain: "_testtttdomain.example",
			expect: true,
		},
		{
			domain: "Top.level-domain",
			expect: true,
		},
		{
			domain: "01domain",
			expect: true,
		},
		{
			domain: "-sample.domain",
			expect: false,
		},
		{
			domain: `%x%sdomain`,
			expect: false,
		},
	}

	for _, c := range cases {
		if c.expect != isValidDomain(c.domain) {
			t.Errorf("Failed to validate : %s", c.domain)
		}
	}
}

func TestNewSvcCertService(t *testing.T) {
	var defaultExpiry int32

	type args struct {
		cfg   config.Config
		token ntokend.TokenProvider
	}
	type test struct {
		name      string
		args      args
		want      SvcCertService
		wantErr   error
		checkfunc func(*svcCertService, *svcCertService) bool
	}

	tests := []test{
		func() test {
			dur, _ := time.ParseDuration("30m")
			token := func() (string, error) { return "", nil }

			return test{
				name: "Success to initialize SvcCertService",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
						},
					},
					token: token,
				},
				want: &svcCertService{
					cfg: config.ServiceCert{
						Enable:        true,
						AthenzCAPath:  "../test/data/dummyCa.pem",
						RefreshPeriod: "30m",
					},
					refreshRequest: &requestTemplate{
						req: &zts.InstanceRefreshRequest{
							ExpiryTime: &defaultExpiry,
						},
					},
					token:           token,
					refreshDuration: dur,
				},
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
				wantErr: nil,
			}
		}(),
		func() test {
			dur, _ := time.ParseDuration("30m")
			token := func() (string, error) { return "", nil }

			var largeExpiry int64 = 2557920      // possible max expiry hour value (292y * 365d * 24h = 2557920h)
			var expectedExpiry int32 = 153475200 // possible max expiry minutes value (largeExpiry hour * 60m = 153475200m)

			return test{
				name: "Success to initialize SvcCertService when it is max expiry of certificate",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
							Expiry:        fmt.Sprintf("%dh", largeExpiry),
						},
					},
					token: token,
				},
				want: &svcCertService{
					cfg: config.ServiceCert{
						Enable:        true,
						AthenzCAPath:  "../test/data/dummyCa.pem",
						RefreshPeriod: "30m",
						Expiry:        fmt.Sprintf("%dh", largeExpiry),
					},
					refreshRequest: &requestTemplate{
						req: &zts.InstanceRefreshRequest{
							ExpiryTime: &expectedExpiry,
						},
					},
					token:           token,
					refreshDuration: dur,
				},
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
				wantErr: nil,
			}
		}(),
		func() test {
			dur, _ := time.ParseDuration("30m")
			token := func() (string, error) { return "", nil }

			var exceedExpiry int64 = 2566680 // exceed max expiry hour value (293y * 365d * 24h = 2557920h)
			var expectedExpiry int32         // when expiry parse error, defaultSvcCertExpiry is 0

			return test{
				name: "Success to initialize SvcCertService using default when the maximum expiry value is exceeded",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
							Expiry:        fmt.Sprintf("%dh", exceedExpiry),
						},
					},
					token: token,
				},
				want: &svcCertService{
					cfg: config.ServiceCert{
						Enable:        true,
						AthenzCAPath:  "../test/data/dummyCa.pem",
						RefreshPeriod: "30m",
						Expiry:        fmt.Sprintf("%dh", exceedExpiry),
					},
					refreshRequest: &requestTemplate{
						req: &zts.InstanceRefreshRequest{
							ExpiryTime: &expectedExpiry,
						},
					},
					token:           token,
					refreshDuration: dur,
				},
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
				wantErr: nil,
			}
		}(),
		func() test {
			dur, _ := time.ParseDuration("30m")
			beforeDur, _ := time.ParseDuration("1h")
			token := func() (string, error) { return "", nil }

			return test{
				name: "Success to initialize SvcCertService with expiry margin",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
							ExpiryMargin:  "1h",
						},
					},
					token: token,
				},
				want: &svcCertService{
					cfg: config.ServiceCert{
						Enable:        true,
						AthenzCAPath:  "../test/data/dummyCa.pem",
						RefreshPeriod: "30m",
						ExpiryMargin:  "1h",
					},
					refreshRequest: &requestTemplate{
						req: &zts.InstanceRefreshRequest{
							ExpiryTime: &defaultExpiry,
						},
					},
					token:           token,
					refreshDuration: dur,
					expireMargin:    beforeDur,
				},
				checkfunc: func(actual, expected *svcCertService) bool {
					return actual.expireMargin == expected.expireMargin
				},
				wantErr: nil,
			}
		}(),
		func() test {
			dur, _ := time.ParseDuration("30m")
			token := func() (string, error) { return "", nil }
			var expiry int32 = 117

			return test{
				name: "Success to initialize SvcCertService with expiry",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
							Expiry:        fmt.Sprintf("%dm", expiry),
						},
					},
					token: token,
				},
				want: &svcCertService{
					cfg: config.ServiceCert{
						Enable:        true,
						AthenzCAPath:  "../test/data/dummyCa.pem",
						RefreshPeriod: "30m",
						Expiry:        fmt.Sprintf("%dm", expiry),
					},
					refreshRequest: &requestTemplate{
						req: &zts.InstanceRefreshRequest{
							ExpiryTime: &expiry,
						},
					},
					token:           token,
					refreshDuration: dur,
				},
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
				wantErr: nil,
			}
		}(),
		func() test {
			dur, _ := time.ParseDuration("30m")
			token := func() (string, error) { return "", nil }

			return test{
				name: "Success to initialize SvcCertService using EC PRIVATE KEY",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyECServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
						},
					},
					token: token,
				},
				want: &svcCertService{
					cfg: config.ServiceCert{
						Enable:        true,
						AthenzCAPath:  "../test/data/dummyCa.pem",
						RefreshPeriod: "30m",
					},
					refreshRequest: &requestTemplate{
						req: &zts.InstanceRefreshRequest{
							ExpiryTime: &defaultExpiry,
						},
					},
					token:           token,
					refreshDuration: dur,
				},
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
				wantErr: nil,
			}
		}(),
		func() test {
			dur, _ := time.ParseDuration("30m")
			token := func() (string, error) { return "", nil }

			return test{
				name: "Success to initialize SvcCertService when spiffe is true",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
							Spiffe:        true,
						},
					},
					token: token,
				},
				want: &svcCertService{
					cfg: config.ServiceCert{
						Enable:        true,
						AthenzCAPath:  "../test/data/dummyCa.pem",
						RefreshPeriod: "30m",
						Spiffe:        true,
					},
					refreshRequest: &requestTemplate{
						req: &zts.InstanceRefreshRequest{
							ExpiryTime: &defaultExpiry,
						},
					},
					token:           token,
					refreshDuration: dur,
				},
				checkfunc: func(actual, expected *svcCertService) bool {
					return actual.cfg.Spiffe == expected.cfg.Spiffe
				},
				wantErr: nil,
			}
		}(),
		func() test {
			token := func() (string, error) { return "", nil }

			return test{
				name: "Fail to parse RefreshPeriod",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "",
						},
					},
					token: token,
				},
				want: &svcCertService{
					cfg: config.ServiceCert{
						Enable:        true,
						AthenzCAPath:  "../test/data/dummyCa.pem",
						RefreshPeriod: "",
					},
					refreshRequest: &requestTemplate{
						req: &zts.InstanceRefreshRequest{
							ExpiryTime: &defaultExpiry,
						},
					},
					token:           token,
					refreshDuration: defaultSvcCertRefreshPeriod,
				},
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
				wantErr: nil,
			}
		}(),
		func() test {
			dur, _ := time.ParseDuration("30m")
			token := func() (string, error) { return "", nil }

			return test{
				name: "Fail to parse expiry margin",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
							ExpiryMargin:  "error",
						},
					},
					token: token,
				},

				want: &svcCertService{
					cfg: config.ServiceCert{
						Enable:        true,
						AthenzCAPath:  "../test/data/dummyCa.pem",
						RefreshPeriod: "30m",
						ExpiryMargin:  "error",
					},
					refreshRequest: &requestTemplate{
						req: &zts.InstanceRefreshRequest{
							ExpiryTime: &defaultExpiry,
						},
					},
					token:           token,
					refreshDuration: dur,
					expireMargin:    defaultSvcCertExpiryMargin,
				},
				checkfunc: func(actual, expected *svcCertService) bool {
					return actual.expireMargin == expected.expireMargin
				},
				wantErr: nil,
			}
		}(),
		func() test {
			token := func() (string, error) { return "", nil }

			return test{
				name: "Private key file does not exist",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/non_exist.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
						},
					},
					token: token,
				},
				want:    &svcCertService{},
				wantErr: ErrLoadPrivateKey,
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
			}
		}(),
		func() test {
			token := func() (string, error) { return "", nil }

			return test{
				name: "Private key is invalid",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/invalid_dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
						},
					},
					token: token,
				},
				want:    &svcCertService{},
				wantErr: ErrFailedToInitialize,
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
			}
		}(),
		func() test {
			token := func() (string, error) { return "", nil }

			return test{
				name: "AthenzCAPath file does not exist",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/non_exist.pem",
							RefreshPeriod: "30m",
						},
					},
					token: token,
				},
				want:    &svcCertService{},
				wantErr: ErrFailedToInitialize,
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
			}
		}(),
		func() test {
			token := func() (string, error) { return "", nil }

			return test{
				name: "Invalid Athenz URL",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "test.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							AthenzURL:     "%2This is not URL",
							RefreshPeriod: "30m",
						},
					},
					token: token,
				},
				want:    &svcCertService{},
				wantErr: ErrFailedToInitialize,
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
			}
		}(),
		func() test {
			token := func() (string, error) { return "", nil }

			return test{
				name: "Invalid Athenz Domain",
				args: args{
					cfg: config.Config{
						NToken: config.NToken{
							AthenzDomain:   "+_invalid.domain",
							PrivateKeyPath: "../test/data/dummyServer.key",
						},
						ServiceCert: config.ServiceCert{
							Enable:        true,
							AthenzCAPath:  "../test/data/dummyCa.pem",
							RefreshPeriod: "30m",
						},
					},
					token: token,
				},
				want:    &svcCertService{},
				wantErr: ErrInvalidParameter,
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
			}
		}(),
		func() test {
			return test{
				name: "SvcCertService disabled",
				args: args{
					cfg: config.Config{
						ServiceCert: config.ServiceCert{},
					},
				},
				want:    nil,
				wantErr: ErrDisabled,
				checkfunc: func(actual, expected *svcCertService) bool {
					return true
				},
			}
		}(),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := NewSvcCertService(tt.args.cfg, tt.args.token)

			if err != tt.wantErr {
				t.Errorf("expected error: %v, actual error: %v", tt.wantErr, err)
			}
			if err != nil {
				return
			}

			actualSvcCertService := actual.(*svcCertService)
			expectedSvcCertService := tt.want.(*svcCertService)

			if actualSvcCertService.cfg != expectedSvcCertService.cfg {
				t.Errorf("Config value is not matched: expected: %+v, actual: %+v", expectedSvcCertService, actualSvcCertService)
			}

			{
				actualValue := actualSvcCertService.refreshDuration
				expectedValue := expectedSvcCertService.refreshDuration
				if actualValue != expectedValue {
					t.Errorf("RefreshPeriod is not matched: expected: %+v, actual: %+v", expectedValue, actualValue)
				}
			}

			{
				actualValue := *actualSvcCertService.refreshRequest.req.ExpiryTime
				expectedValue := *expectedSvcCertService.refreshRequest.req.ExpiryTime
				if actualValue != expectedValue {
					t.Errorf("expected error: %v, actual error: %v", expectedValue, actualValue)
				}
			}

			if !tt.checkfunc(actualSvcCertService, expectedSvcCertService) {
				t.Errorf("Individual test failed expected: %+v, actual: %+v", expectedSvcCertService, actualSvcCertService)
			}
		})
	}
}

func TestSvcCertService_GetSvcCertProvider(t *testing.T) {
	svcCertService, _ := NewSvcCertService(
		config.Config{
			NToken: config.NToken{
				AthenzDomain:   "test.domain",
				PrivateKeyPath: "../test/data/dummyServer.key",
			},
			ServiceCert: config.ServiceCert{
				Enable:        true,
				AthenzCAPath:  "../test/data/dummyCa.pem",
				RefreshPeriod: "30m",
			},
		},
		func() (string, error) { return "N-token", nil },
	)

	if svcCertService.GetSvcCertProvider() == nil {
		t.Error("GetSvcCertProvider is nil")
	}
}

// mockTransporter is the mock of RoundTripper
type mockTransporter struct {
	StatusCode int
	Body       [][]byte
	Method     string
	URL        *url.URL
	Error      error
	Counter    int
}

// RoundTrip is used to create a mock http response
func (m *mockTransporter) RoundTrip(req *http.Request) (*http.Response, error) {
	m.Counter = m.Counter + 1
	res := &http.Response{
		Status:     fmt.Sprintf("%d %s", m.StatusCode, http.StatusText(m.StatusCode)),
		StatusCode: m.StatusCode,
		Body:       ioutil.NopCloser(bytes.NewBuffer(nil)),
		Request: &http.Request{
			URL:    m.URL,
			Method: m.Method,
		},
	}
	if m.Counter > len(m.Body) {
		// prevent extra update request causing array index panic
		return res, nil
	}
	res.Body = ioutil.NopCloser(bytes.NewBuffer(m.Body[m.Counter-1]))
	return res, m.Error
}

func TestSvcCertService_GetSvcCert(t *testing.T) {
	type test struct {
		name           string
		svcCertService SvcCertService
		want           []byte
		wantErr        error
	}

	tests := []test{
		func() test {
			dummyCertBytes, _ := ioutil.ReadFile("../test/data/dummyServer.crt")
			dummyCaCertBytes, _ := ioutil.ReadFile("../test/data/dummyCa.pem")
			dummyCert := strings.ReplaceAll(string(dummyCertBytes), "\n", "\\n")
			dummyCaCert := strings.ReplaceAll(string(dummyCaCertBytes), "\n", "\\n")

			dummyResponce := fmt.Sprintf(
				`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCert, dummyCaCert,
			)
			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       [][]byte{[]byte(dummyResponce)},
				Method:     "GET",
				Error:      nil,
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    true,
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			svcCertService.client.Transport = transpoter
			cache := certCache{
				cert: dummyCertBytes,
				exp:  fastime.Now().Add(time.Hour),
			}
			svcCertService.certCache.Store(cache)

			return test{
				name:           "getSvcCert returns stored value.",
				svcCertService: svcCertService,
				want:           dummyCertBytes,
				wantErr:        nil,
			}
		}(),
		func() test {
			dummyCertBytes, _ := ioutil.ReadFile("../test/data/dummyServer.crt")
			dummyCaCertBytes, _ := ioutil.ReadFile("../test/data/dummyCa.pem")
			dummyCert := strings.ReplaceAll(string(dummyCertBytes), "\n", "\\n")
			dummyCaCert := strings.ReplaceAll(string(dummyCaCertBytes), "\n", "\\n")

			dummyResponce := fmt.Sprintf(
				`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCert, dummyCaCert,
			)
			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       [][]byte{[]byte(dummyResponce)},
				Method:     "GET",
				Error:      nil,
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    true,
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			svcCertService.client.Transport = transpoter

			return test{
				name:           "getSvcCert returns value from RefreshSvcCert",
				svcCertService: svcCertService,
				want:           append(dummyCertBytes, dummyCaCertBytes...),
				wantErr:        nil,
			}
		}(),
		func() test {
			dummyCertBytes, _ := ioutil.ReadFile("../test/data/dummyServer.crt")
			dummyCaCertBytes, _ := ioutil.ReadFile("../test/data/dummyCa.pem")
			dummyCert := strings.ReplaceAll(string(dummyCertBytes), "\n", "\\n")
			dummyCaCert := strings.ReplaceAll(string(dummyCaCertBytes), "\n", "\\n")

			dummyResponce := fmt.Sprintf(
				`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCert, dummyCaCert,
			)
			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       [][]byte{[]byte(dummyResponce)},
				Method:     "GET",
				Error:      nil,
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    true,
					ExpiryMargin:        "1000h",
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			cache := certCache{
				cert: dummyCertBytes,
				exp:  fastime.Now().Add(-time.Hour),
			}
			svcCertService.certCache.Store(cache)

			svcCertService.client.Transport = transpoter

			return test{
				name:           "getSvcCert returns value from RefreshSvcCert when expiry is before now.",
				svcCertService: svcCertService,
				want:           append(dummyCertBytes, dummyCaCertBytes...),
				wantErr:        nil,
			}
		}(),
		func() test {
			dummyCertBytes, _ := ioutil.ReadFile("../test/data/dummyServer.crt")
			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 500,
				Body:       [][]byte{{}},
				Method:     "GET",
				Error:      fmt.Errorf("request error"),
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    true,
					ExpiryMargin:        "10d",
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			cache := certCache{
				cert: dummyCertBytes,
				exp:  fastime.Now().Add(-time.Hour),
			}
			svcCertService.certCache.Store(cache)

			svcCertService.client.Transport = transpoter

			return test{
				name:           "getSvcCert returns value from cache when failed to refresh and Not After of cache is after now.",
				svcCertService: svcCertService,
				want:           dummyCertBytes,
				wantErr:        nil,
			}
		}(),
		func() test {
			dummyCertBytes, _ := ioutil.ReadFile("../test/data/expired_dummyServer.crt")

			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 500,
				Body:       [][]byte{{}},
				Method:     "GET",
				Error:      fmt.Errorf("request error"),
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    true,
					ExpiryMargin:        "1s",
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			cache := certCache{
				cert: dummyCertBytes,
				exp:  fastime.Now().Add(-time.Hour),
			}
			svcCertService.certCache.Store(cache)

			svcCertService.client.Transport = transpoter

			wantErr := fmt.Errorf(
				`Post "%s/instance/%s/%s/refresh": request error`,
				cfg.ServiceCert.AthenzURL,
				cfg.NToken.AthenzDomain,
				cfg.NToken.ServiceName,
			)

			return test{
				name:           "getSvcCert returns error when failed to refresh and cached cert is expired.",
				svcCertService: svcCertService,
				want:           nil,
				wantErr:        wantErr,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.svcCertService.(*svcCertService)
			cert, err := s.getSvcCert()

			if tt.wantErr == nil && err != nil {
				t.Errorf("failed to instantiate, err: %v", err)
			} else if tt.wantErr != nil {
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("error not the same, want: %v, got: %v", tt.wantErr, err)
				}
			} else if string(tt.want) != string(cert) {
				t.Errorf("RefreshSvcCert got: %v, want: %v", string(cert), string(tt.want))
			}
		})
	}
}

func TestSvcCertService_StartSvcCertUpdater(t *testing.T) {
	type test struct {
		name           string
		svcCertService SvcCertService
		checkFunc      func(*svcCertService, *testing.T)
		afterFunc      func()
	}

	tests := []test{
		func() test {
			ctx, cancel := context.WithCancel(context.Background())

			dummyCertBytes, _ := ioutil.ReadFile("../test/data/dummyServer.crt")
			dummyCaCertBytes, _ := ioutil.ReadFile("../test/data/dummyCa.pem")
			dummyCert := strings.ReplaceAll(string(dummyCertBytes), "\n", "\\n")
			dummyCaCert := strings.ReplaceAll(string(dummyCaCertBytes), "\n", "\\n")

			dummyResponce := [][]byte{
				[]byte(fmt.Sprintf(`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCert, dummyCaCert)),
				[]byte(fmt.Sprintf(`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCaCert, dummyCaCert)),
			}

			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       dummyResponce,
				Method:     "GET",
				Error:      nil,
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "100ms",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    true,
				},
			}

			checkFunc := func(s *svcCertService, t *testing.T) {
				s.StartSvcCertUpdater(ctx)
				cert1, _ := s.GetSvcCertProvider()()
				time.Sleep(time.Millisecond * 120)
				cert2, _ := s.GetSvcCertProvider()()
				if string(cert1) == string(cert2) {
					t.Errorf("cert did not refreshed")
				}
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			svcCertService.client.Transport = transpoter

			return test{
				name:           "cert is refreshed successfully",
				svcCertService: svcCertService,
				checkFunc:      checkFunc,
				afterFunc:      cancel,
			}
		}(),
		func() test {
			ctx, cancel := context.WithCancel(context.Background())

			dummyCertBytes, _ := ioutil.ReadFile("../test/data/dummyServer.crt")
			dummyCaCertBytes, _ := ioutil.ReadFile("../test/data/dummyCa.pem")
			dummyCert := strings.ReplaceAll(string(dummyCertBytes), "\n", "\\n")
			dummyCaCert := strings.ReplaceAll(string(dummyCaCertBytes), "\n", "\\n")

			dummyResponce := [][]byte{
				[]byte(fmt.Sprintf(`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCert, dummyCaCert)),
				[]byte(fmt.Sprintf(`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCaCert, dummyCaCert)),
			}

			token := func() (string, error) { return "", fmt.Errorf("error") }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       dummyResponce,
				Method:     "GET",
				Error:      nil,
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "100ms",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    true,
				},
			}

			checkFunc := func(s *svcCertService, t *testing.T) {
				s.StartSvcCertUpdater(ctx)
				cert1, _ := s.GetSvcCertProvider()()
				time.Sleep(time.Millisecond * 250)
				cert2, _ := s.GetSvcCertProvider()()
				if string(cert1) != string(cert2) {
					t.Errorf("cert refreshed")
				}
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			svcCertService.client.Transport = transpoter

			return test{
				name:           "fail to refresh cert when RefreshSvcCert returns error",
				svcCertService: svcCertService,
				checkFunc:      checkFunc,
				afterFunc:      cancel,
			}
		}(),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.svcCertService.(*svcCertService)

			defer tt.afterFunc()

			if tt.checkFunc == nil {
				t.Errorf("checkfunc is nil")
			} else {
				tt.checkFunc(s, t)
			}
		})
	}

}

func TestSvcCertService_RefreshSvcCert(t *testing.T) {
	type test struct {
		name           string
		svcCertService SvcCertService
		want           []byte
		wantErr        error
	}

	tests := []test{
		func() test {
			dummyCertBytes, _ := ioutil.ReadFile("../test/data/dummyServer.crt")
			dummyCaCertBytes, _ := ioutil.ReadFile("../test/data/dummyCa.pem")
			dummyCert := strings.ReplaceAll(string(dummyCertBytes), "\n", "\\n")
			dummyCaCert := strings.ReplaceAll(string(dummyCaCertBytes), "\n", "\\n")

			dummyResponce := fmt.Sprintf(
				`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCert, dummyCaCert,
			)
			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       [][]byte{[]byte(dummyResponce)},
				Method:     "GET",
				Error:      nil,
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    true,
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			svcCertService.client.Transport = transpoter

			return test{
				name:           "RefreshSvcCert returns correct when IntermediateCert is true",
				svcCertService: svcCertService,
				want:           append(dummyCertBytes, dummyCaCertBytes...),
				wantErr:        nil,
			}
		}(),
		func() test {
			dummyCertBytes, _ := ioutil.ReadFile("../test/data/dummyServer.crt")
			dummyCaCertBytes, _ := ioutil.ReadFile("../test/data/dummyCa.pem")
			dummyCert := strings.ReplaceAll(string(dummyCertBytes), "\n", "\\n")
			dummyCaCert := strings.ReplaceAll(string(dummyCaCertBytes), "\n", "\\n")

			dummyResponce := fmt.Sprintf(
				`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCert, dummyCaCert,
			)
			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       [][]byte{[]byte(dummyResponce)},
				Method:     "GET",
				Error:      nil,
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    false,
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			svcCertService.client.Transport = transpoter

			return test{
				name:           "RefreshSvcCert returns correct when IntermediateCert is false",
				svcCertService: svcCertService,
				want:           dummyCertBytes,
				wantErr:        nil,
			}
		}(),
		func() test {
			dummyCertBytes, _ := ioutil.ReadFile("../test/data/dummyServer.crt")
			dummyCaCertBytes, _ := ioutil.ReadFile("../test/data/dummyCa.pem")
			dummyCert := strings.ReplaceAll(string(dummyCertBytes), "\n", "\\n")
			dummyCaCert := strings.ReplaceAll(string(dummyCaCertBytes), "\n", "\\n")

			dummyResponce := fmt.Sprintf(
				`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCert, dummyCaCert,
			)
			token := func() (string, error) { return "", fmt.Errorf("N-token error") }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       [][]byte{[]byte(dummyResponce)},
				Method:     "GET",
				Error:      nil,
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    false,
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			svcCertService.client.Transport = transpoter

			return test{
				name:           "RefreshSvcCert fail when ntokend returns error",
				svcCertService: svcCertService,
				want:           nil,
				wantErr:        fmt.Errorf("N-token error"),
			}
		}(),
		func() test {
			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       [][]byte{[]byte("{}")},
				Method:     "GET",
				Error:      fmt.Errorf("request error"),
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    false,
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			svcCertService.client.Transport = transpoter

			wantErr := fmt.Errorf(
				`Post "%s/instance/%s/%s/refresh": request error`,
				cfg.ServiceCert.AthenzURL,
				cfg.NToken.AthenzDomain,
				cfg.NToken.ServiceName,
			)

			return test{
				name:           "RefreshSvcCert fail when request failed",
				svcCertService: svcCertService,
				want:           nil,
				wantErr:        wantErr,
			}
		}(),
		func() test {
			dummyCertBytes, _ := ioutil.ReadFile("../test/data/invalid_dummyServer.crt")
			dummyCaCertBytes, _ := ioutil.ReadFile("../test/data/dummyCa.pem")
			dummyCert := strings.ReplaceAll(string(dummyCertBytes), "\n", "\\n")
			dummyCaCert := strings.ReplaceAll(string(dummyCaCertBytes), "\n", "\\n")

			dummyResponce := fmt.Sprintf(
				`{"name": "dummy", "certificate":"%s", "caCertBundle": "%s"}`, dummyCert, dummyCaCert,
			)
			token := func() (string, error) { return "dummyToken", nil }

			transpoter := &mockTransporter{
				StatusCode: 200,
				Body:       [][]byte{[]byte(dummyResponce)},
				Method:     "GET",
				Error:      nil,
			}

			cfg := config.Config{
				NToken: config.NToken{
					PrivateKeyPath: "../test/data/dummyServer.key",
					AthenzDomain:   "dummyDomain",
					ServiceName:    "dummyService",
				},
				ServiceCert: config.ServiceCert{
					Enable:              true,
					AthenzCAPath:        "../test/data/dummyCa.pem",
					AthenzURL:           "http://dummy",
					RefreshPeriod:       "30m",
					PrincipalAuthHeader: "Athenz-Principal",
					IntermediateCert:    false,
				},
			}

			s, _ := NewSvcCertService(cfg, token)
			svcCertService := s.(*svcCertService)

			svcCertService.client.Transport = transpoter

			return test{
				name:           "RefreshSvcCert fail when recieved cert is invalid",
				svcCertService: svcCertService,
				want:           nil,
				wantErr:        ErrInvalidCert,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.svcCertService.(*svcCertService)
			cert, err := s.RefreshSvcCert()

			if tt.wantErr == nil && err != nil {
				t.Errorf("failed to instantiate, err: %v", err)
			} else if tt.wantErr != nil {
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("error not the same, want: %v, got: %v", tt.wantErr, err)
				}
			} else if string(tt.want) != string(cert) {
				t.Errorf("RefreshSvcCert got: %v, want: %v", string(cert), tt.want)
			}
		})
	}
}
