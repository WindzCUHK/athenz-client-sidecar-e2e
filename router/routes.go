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

package router

import (
	"net/http"

	"github.com/AthenZ/athenz-client-sidecar/v2/config"
	"github.com/AthenZ/athenz-client-sidecar/v2/handler"
)

// Route manages the routing for sidecar.
type Route struct {
	Name        string
	Methods     []string
	Pattern     string
	HandlerFunc handler.Func
}

// NewRoutes returns Route slice.
func NewRoutes(cfg config.Config, h handler.Handler) []Route {
	var r []Route

	if cfg.NToken.Enable {
		r = append(r, Route{
			"NToken Handler",
			[]string{
				http.MethodGet,
			},
			"/ntoken",
			h.NToken,
		})
	}

	if cfg.AccessToken.Enable {
		r = append(r, Route{
			"Access Token Handler",
			[]string{
				http.MethodPost,
			},
			"/accesstoken",
			h.AccessToken,
		})
	}

	if cfg.RoleToken.Enable {
		r = append(r, Route{
			"RoleToken Handler",
			[]string{
				http.MethodPost,
			},
			"/roletoken",
			h.RoleToken,
		})
	}

	if cfg.ServiceCert.Enable {
		r = append(r, Route{
			"Service Cert Handler",
			[]string{
				http.MethodGet,
			},
			"/svccert",
			h.ServiceCert,
		})
	}

	if cfg.Proxy.Enable {
		r = append(r, Route{
			"RoleToken proxy Handler",
			[]string{
				"*",
			},
			"/proxy/roletoken",
			h.RoleTokenProxy,
		}, Route{
			"NToken proxy Handler",
			[]string{
				"*",
			},
			"/proxy/ntoken",
			h.NTokenProxy,
		})
	}

	return r
}
