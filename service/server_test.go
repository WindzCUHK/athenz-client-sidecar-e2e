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
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AthenZ/athenz-client-sidecar/v2/config"
)

func TestNewServer(t *testing.T) {
	type args struct {
		opts []Option
	}
	tests := []struct {
		name      string
		args      args
		want      Server
		checkFunc func(got, want Server) error
	}{
		{
			name: "Check health address",
			args: args{
				opts: []Option{
					WithServerConfig(config.Server{
						HealthCheck: config.HealthCheck{
							Address:  "48.48.48.48",
							Port:     8080,
							Endpoint: "/healthz",
						},
					}),
					WithServerHandler(func() http.Handler {
						return nil
					}()),
				},
			},
			want: &server{
				hcsrv: &http.Server{
					Addr: fmt.Sprintf("%s:%d", "48.48.48.48", 8080),
				},
			},
			checkFunc: func(got, want Server) error {
				if got.(*server).hcsrv.Addr != want.(*server).hcsrv.Addr {
					return fmt.Errorf("Healthz Addr not equals\tgot: %s\twant: %s", got.(*server).hcsrv.Addr, want.(*server).hcsrv.Addr)
				}
				return nil
			},
		},
		{
			name: "Check server address",
			args: args{
				opts: []Option{
					WithServerConfig(config.Server{
						Address: "75.75.75.75",
						Port:    8081,
						HealthCheck: config.HealthCheck{
							Port:     8080,
							Endpoint: "/healthz",
						},
					}),
					WithServerHandler(func() http.Handler {
						return nil
					}()),
				},
			},
			want: &server{
				srv: &http.Server{
					Addr: fmt.Sprintf("%s:%d", "75.75.75.75", 8081),
				},
			},
			checkFunc: func(got, want Server) error {
				if got.(*server).srv.Addr != want.(*server).srv.Addr {
					return fmt.Errorf("Server Addr not equals\tgot: %s\twant: %s", got.(*server).srv.Addr, want.(*server).srv.Addr)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewServer(tt.args.opts...)
			if err := tt.checkFunc(got, tt.want); err != nil {
				t.Errorf("NewServer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_server_ListenAndServe(t *testing.T) {
	type fields struct {
		srv   *http.Server
		hcsrv *http.Server
		cfg   config.Server
	}
	type args struct {
		ctx context.Context
	}
	type test struct {
		name       string
		fields     fields
		args       args
		beforeFunc func() error
		checkFunc  func(*server, chan []error, error) error
		afterFunc  func() error
		want       error
	}

	checkSrvRunning := func(addr string) error {
		res, err := http.DefaultClient.Get(addr)
		if err != nil {
			return err
		}
		if res.StatusCode != 200 {
			return fmt.Errorf("Response status code invalid, %v", res.StatusCode)
		}
		return nil
	}

	tests := []test{
		func() test {
			ctx, cancelFunc := context.WithCancel(context.Background())

			keyKey := "_dummy_key_"
			key := "../test/data/dummyServer.key"
			certKey := "_dummy_cert_"
			cert := "../test/data/dummyServer.crt"

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				fmt.Fprintln(w, "Hello, client")
			})

			apiSrvPort := 9998
			hcSrvPort := 9999
			apiSrvAddr := fmt.Sprintf("https://127.0.0.1:%v", apiSrvPort)
			hcSrvAddr := fmt.Sprintf("http://127.0.0.1:%v", hcSrvPort)

			return test{
				name: "Test servers can start and stop",
				fields: fields{
					srv: func() *http.Server {
						s := &http.Server{
							Addr:    fmt.Sprintf("localhost:%d", apiSrvPort),
							Handler: handler,
						}
						s.SetKeepAlivesEnabled(true)
						return s
					}(),
					hcsrv: func() *http.Server {
						s := &http.Server{
							Addr:    fmt.Sprintf("localhost:%d", hcSrvPort),
							Handler: handler,
						}
						s.SetKeepAlivesEnabled(true)
						return s
					}(),
					cfg: config.Server{
						Port: apiSrvPort,
						TLS: config.TLS{
							Enable:   true,
							CertPath: certKey,
							KeyPath:  keyKey,
						},
						HealthCheck: config.HealthCheck{
							Port: hcSrvPort,
						},
					},
				},
				args: args{
					ctx: ctx,
				},
				beforeFunc: func() error {
					if err := os.Setenv(strings.TrimPrefix(strings.TrimSuffix(keyKey, "_"), "_"), key); err != nil {
						return err
					}
					return os.Setenv(strings.TrimPrefix(strings.TrimSuffix(certKey, "_"), "_"), cert)
				},
				checkFunc: func(s *server, got chan []error, want error) error {
					time.Sleep(time.Millisecond * 150)
					http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

					if err := checkSrvRunning(apiSrvAddr); err != nil {
						return fmt.Errorf("Server not running, err: %v", err)
					}

					if err := checkSrvRunning(hcSrvAddr); err != nil {
						return fmt.Errorf("Health Check server not running")
					}

					cancelFunc()
					time.Sleep(time.Millisecond * 250)

					if err := checkSrvRunning(apiSrvAddr); err == nil {
						return fmt.Errorf("Server running")
					}

					if err := checkSrvRunning(hcSrvAddr); err == nil {
						return fmt.Errorf("Health Check server running")
					}

					return nil
				},
				afterFunc: func() error {
					cancelFunc()
					if err := os.Unsetenv(strings.TrimPrefix(strings.TrimSuffix(keyKey, "_"), "_")); err != nil {
						return err
					}
					if err := os.Unsetenv(strings.TrimPrefix(strings.TrimSuffix(certKey, "_"), "_")); err != nil {
						return nil
					}
					return nil
				},
			}
		}(),
		func() test {
			keyKey := "_dummy_key_"
			key := "../test/data/dummyServer.key"
			certKey := "_dummy_cert_"
			cert := "../test/data/dummyServer.crt"

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				fmt.Fprintln(w, "Hello, client")
			})

			apiSrvPort := 9998
			hcSrvPort := 9999
			apiSrvAddr := fmt.Sprintf("https://127.0.0.1:%v", apiSrvPort)
			hcSrvAddr := fmt.Sprintf("http://127.0.0.1:%v", hcSrvPort)

			return test{
				name: "Test HC server stop when api server stop",
				fields: fields{
					srv: func() *http.Server {
						srv := &http.Server{
							Addr:    fmt.Sprintf("localhost:%d", apiSrvPort),
							Handler: handler,
						}

						srv.SetKeepAlivesEnabled(true)
						return srv
					}(),
					hcsrv: func() *http.Server {
						srv := &http.Server{
							Addr:    fmt.Sprintf("localhost:%d", hcSrvPort),
							Handler: handler,
						}

						srv.SetKeepAlivesEnabled(true)
						return srv
					}(),
					cfg: config.Server{
						Port: apiSrvPort,
						TLS: config.TLS{
							Enable:   true,
							CertPath: certKey,
							KeyPath:  keyKey,
						},
						HealthCheck: config.HealthCheck{
							Port: hcSrvPort,
						},
					},
				},
				args: args{
					ctx: context.Background(),
				},
				beforeFunc: func() error {
					if err := os.Setenv(strings.TrimPrefix(strings.TrimSuffix(keyKey, "_"), "_"), key); err != nil {
						return err
					}
					return os.Setenv(strings.TrimPrefix(strings.TrimSuffix(certKey, "_"), "_"), cert)
				},
				checkFunc: func(s *server, got chan []error, want error) error {
					time.Sleep(time.Millisecond * 150)
					http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

					if err := checkSrvRunning(apiSrvAddr); err != nil {
						return fmt.Errorf("Server not running, err: %v", err)
					}

					if err := checkSrvRunning(hcSrvAddr); err != nil {
						return fmt.Errorf("Health Check server not running")
					}

					s.srv.Close()
					time.Sleep(time.Millisecond * 150)

					if err := checkSrvRunning(apiSrvAddr); err == nil {
						return fmt.Errorf("Server running")
					}

					if err := checkSrvRunning(hcSrvAddr); err == nil {
						return fmt.Errorf("Health Check server running")
					}

					return nil
				},
				afterFunc: func() error {
					if err := os.Unsetenv(strings.TrimPrefix(strings.TrimSuffix(keyKey, "_"), "_")); err != nil {
						return err
					}
					if err := os.Unsetenv(strings.TrimPrefix(strings.TrimSuffix(certKey, "_"), "_")); err != nil {
						return nil
					}
					return nil
				},
			}
		}(),

		func() test {
			keyKey := "_dummy_key_"
			key := "../test/data/dummyServer.key"
			certKey := "_dummy_cert_"
			cert := "../test/data/dummyServer.crt"

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				fmt.Fprintln(w, "Hello, client")
			})

			apiSrvPort := 9998
			hcSrvPort := 9999
			apiSrvAddr := fmt.Sprintf("https://127.0.0.1:%v", apiSrvPort)
			hcSrvAddr := fmt.Sprintf("http://127.0.0.1:%v", hcSrvPort)

			return test{
				name: "Test api server stop when HC server stop",
				fields: fields{
					srv: func() *http.Server {
						srv := &http.Server{
							Addr:    fmt.Sprintf("localhost:%d", apiSrvPort),
							Handler: handler,
						}

						srv.SetKeepAlivesEnabled(true)
						return srv
					}(),
					hcsrv: func() *http.Server {
						srv := &http.Server{
							Addr:    fmt.Sprintf("localhost:%d", hcSrvPort),
							Handler: handler,
						}

						srv.SetKeepAlivesEnabled(true)
						return srv
					}(),
					cfg: config.Server{
						Port: apiSrvPort,
						TLS: config.TLS{
							Enable:   true,
							CertPath: certKey,
							KeyPath:  keyKey,
						},
						HealthCheck: config.HealthCheck{
							Port: hcSrvPort,
						},
					},
				},
				args: args{
					ctx: context.Background(),
				},
				beforeFunc: func() error {
					if err := os.Setenv(strings.TrimPrefix(strings.TrimSuffix(keyKey, "_"), "_"), key); err != nil {
						return err
					}
					return os.Setenv(strings.TrimPrefix(strings.TrimSuffix(certKey, "_"), "_"), cert)
				},
				checkFunc: func(s *server, got chan []error, want error) error {
					time.Sleep(time.Millisecond * 150)
					http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

					if err := checkSrvRunning(apiSrvAddr); err != nil {
						return fmt.Errorf("Server not running, err: %v", err)
					}

					if err := checkSrvRunning(hcSrvAddr); err != nil {
						return fmt.Errorf("Health Check server not running")
					}

					s.hcsrv.Close()
					time.Sleep(time.Millisecond * 150)

					if err := checkSrvRunning(apiSrvAddr); err == nil {
						return fmt.Errorf("Server running")
					}

					if err := checkSrvRunning(hcSrvAddr); err == nil {
						return fmt.Errorf("Health Check server running")
					}

					return nil
				},
				afterFunc: func() error {
					if err := os.Unsetenv(strings.TrimPrefix(strings.TrimSuffix(keyKey, "_"), "_")); err != nil {
						return err
					}
					if err := os.Unsetenv(strings.TrimPrefix(strings.TrimSuffix(certKey, "_"), "_")); err != nil {
						return nil
					}
					return nil
				},
			}
		}(),
		func() test {
			ctx, cancelFunc := context.WithCancel(context.Background())
			key := "../test/data/dummyServer.key"
			cert := "../test/data/dummyServer.crt"

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				fmt.Fprintln(w, "Hello, client")
			})

			apiSrvPort := 9998
			hcSrvPort := 9999
			apiSrvAddr := fmt.Sprintf("https://127.0.0.1:%v", apiSrvPort)
			hcSrvAddr := fmt.Sprintf("http://127.0.0.1:%v", hcSrvPort)

			return test{
				name: "Test health check server disable",
				fields: fields{
					srv: func() *http.Server {
						srv := &http.Server{
							Addr:    fmt.Sprintf("localhost:%d", apiSrvPort),
							Handler: handler,
						}

						srv.SetKeepAlivesEnabled(true)
						return srv
					}(),
					hcsrv: func() *http.Server {
						srv := &http.Server{
							Addr:    fmt.Sprintf("localhost:%d", hcSrvPort),
							Handler: handler,
						}

						srv.SetKeepAlivesEnabled(true)
						return srv
					}(),
					cfg: config.Server{
						Port: apiSrvPort,
						TLS: config.TLS{
							Enable:   true,
							CertPath: cert,
							KeyPath:  key,
						},
						HealthCheck: config.HealthCheck{
							// Port: hcSrvPort,
						},
					},
				},
				args: args{
					ctx: ctx,
				},
				checkFunc: func(s *server, got chan []error, want error) error {
					time.Sleep(time.Millisecond * 150)
					http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

					if err := checkSrvRunning(apiSrvAddr); err != nil {
						return fmt.Errorf("Server not running")
					}
					if err := checkSrvRunning(hcSrvAddr); err == nil {
						return fmt.Errorf("Health Check server running")
					}

					cancelFunc()
					time.Sleep(time.Millisecond * 150)

					if err := checkSrvRunning(apiSrvAddr); err == nil {
						return fmt.Errorf("Server running")
					}
					if err := checkSrvRunning(hcSrvAddr); err == nil {
						return fmt.Errorf("Health Check server running")
					}

					return nil
				},
			}
		}(),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.afterFunc != nil {
				defer func() {
					err := tt.afterFunc()
					if err != nil {
						t.Errorf("server.listenAndServe() afterFunc error: %v", err)
					}
				}()
			}
			if tt.beforeFunc != nil {
				if err := tt.beforeFunc(); err != nil {
					t.Errorf("before func error: %v", err)
					return
				}
			}

			s := &server{
				srv:   tt.fields.srv,
				hcsrv: tt.fields.hcsrv,
				cfg:   tt.fields.cfg,
			}

			e := s.ListenAndServe(tt.args.ctx)
			if err := tt.checkFunc(s, e, tt.want); err != nil {
				t.Errorf("server.listenAndServe() Error = %v", err)
			}
		})
	}
}

func Test_server_createHealthCheckServiceMux(t *testing.T) {
	type args struct {
		pattern string
	}
	type test struct {
		name       string
		args       args
		beforeFunc func() error
		checkFunc  func(*http.ServeMux) error
		afterFunc  func() error
	}
	tests := []test{
		func() test {
			return test{
				name: "Test create server mux",
				args: args{
					pattern: ":8080",
				},
				checkFunc: func(got *http.ServeMux) error {
					if got == nil {
						return fmt.Errorf("serveMux is empty")
					}
					return nil
				},
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.afterFunc != nil {
				defer func() {
					if err := tt.afterFunc(); err != nil {
						t.Errorf("afterFunc error, error: %v", err)
						return
					}
				}()
			}
			if tt.beforeFunc != nil {
				err := tt.beforeFunc()
				if err != nil {
					t.Errorf("beforeFunc error, error: %v", err)
					return
				}
			}

			got := createHealthCheckServiceMux(tt.args.pattern)
			if err := tt.checkFunc(got); err != nil {
				t.Errorf("server.listenAndServeAPI() Error = %v", err)
			}
		})
	}
}

func Test_server_handleHealthCheckRequest(t *testing.T) {
	type args struct {
		rw http.ResponseWriter
		r  *http.Request
	}
	type test struct {
		name       string
		args       args
		beforeFunc func() error
		checkFunc  func() error
		afterFunc  func() error
	}
	tests := []test{
		func() test {
			rw := httptest.NewRecorder()

			return test{
				name: "Test handle HTTP GET request health check request",
				args: args{
					rw: rw,
					r:  httptest.NewRequest(http.MethodGet, "/", nil),
				},
				checkFunc: func() error {
					result := rw.Result()
					if header := result.StatusCode; header != http.StatusOK {
						return fmt.Errorf("Header is not correct, got: %v", header)
					}
					if contentType := rw.Header().Get("Content-Type"); contentType != "text/plain;charset=UTF-8" {
						return fmt.Errorf("Content type is not correct, got: %v", contentType)
					}
					return nil
				},
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.afterFunc != nil {
				defer func() {
					if err := tt.afterFunc(); err != nil {
						t.Errorf("%v", err)
						return
					}
				}()
			}

			if tt.beforeFunc != nil {
				err := tt.beforeFunc()
				if err != nil {
					t.Errorf("beforeFunc error, error: %v", err)
					return
				}
			}

			handleHealthCheckRequest(tt.args.rw, tt.args.r)
			if err := tt.checkFunc(); err != nil {
				t.Errorf("error: %v", err)
			}
		})
	}
}

func Test_server_listenAndServeAPI(t *testing.T) {
	type fields struct {
		srv   *http.Server
		hcsrv *http.Server
		cfg   config.Server
	}
	type test struct {
		name       string
		fields     fields
		beforeFunc func() error
		checkFunc  func(*server, error) error
		afterFunc  func() error
		want       error
	}
	tests := []test{
		func() test {
			keyKey := "_dummy_key_"
			key := "../test/data/dummyServer.key"
			certKey := "_dummy_cert_"
			cert := "../test/data/dummyServer.crt"

			return test{
				name: "Test server startup",
				fields: fields{
					srv: &http.Server{
						Handler: func() http.Handler {
							return nil
						}(),
						Addr: fmt.Sprintf("localhost:%d", 9999),
					},
					cfg: config.Server{
						Port: 9999,
						TLS: config.TLS{
							Enable:   true,
							CertPath: certKey,
							KeyPath:  keyKey,
						},
					},
				},
				beforeFunc: func() error {
					err := os.Setenv(strings.TrimPrefix(strings.TrimSuffix(keyKey, "_"), "_"), key)
					if err != nil {
						return err
					}
					err = os.Setenv(strings.TrimPrefix(strings.TrimSuffix(certKey, "_"), "_"), cert)
					if err != nil {
						return err
					}
					return nil
				},
				checkFunc: func(s *server, want error) error {
					// listenAndServeAPI function is blocking, so we need to set timer to shutdown the process
					go func() {
						time.Sleep(time.Millisecond * 100)
						if err := s.srv.Shutdown(context.Background()); err != nil {
							panic(err)
						}
					}()

					got := s.listenAndServeAPI()

					if got != want {
						return fmt.Errorf("got:\t%v\nwant:\t%v", got, want)
					}
					return nil
				},
				afterFunc: func() error {
					os.Unsetenv(strings.TrimPrefix(strings.TrimSuffix(keyKey, "_"), "_"))
					os.Unsetenv(strings.TrimPrefix(strings.TrimSuffix(certKey, "_"), "_"))
					return nil
				},
				want: http.ErrServerClosed,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.afterFunc != nil {
				defer func() {
					if err := tt.afterFunc(); err != nil {
						t.Errorf("%v", err)
						return
					}
				}()
			}

			if tt.beforeFunc != nil {
				err := tt.beforeFunc()
				if err != nil {
					t.Errorf("beforeFunc error, error: %v", err)
					return
				}
			}

			if err := tt.checkFunc(&server{
				srv:   tt.fields.srv,
				hcsrv: tt.fields.hcsrv,
				cfg:   tt.fields.cfg,
			}, tt.want); err != nil {
				t.Errorf("server.listenAndServeAPI() Error = %v", err)
			}
		})
	}
}
