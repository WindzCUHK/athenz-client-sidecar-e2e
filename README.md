# Athenz client sidecar

[![License: Apache](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://opensource.org/licenses/Apache-2.0)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/AthenZ/athenz-client-sidecar?style=flat-square&label=Github%20version)](https://github.com/AthenZ/athenz-client-sidecar/releases/latest)
[![Docker Image Version (tag latest)](https://img.shields.io/docker/v/athenz/athenz-client-sidecar/latest?style=flat-square&label=Docker%20version)](https://hub.docker.com/r/athenz/athenz-client-sidecar/tags)
[![Go Report Card](https://goreportcard.com/badge/github.com/AthenZ/athenz-client-sidecar)](https://goreportcard.com/report/github.com/AthenZ/athenz-client-sidecar)
[![GoDoc](http://godoc.org/github.com/AthenZ/athenz-client-sidecar?status.svg)](http://godoc.org/github.com/AthenZ/athenz-client-sidecar)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](code_of_conduct.md)

![logo](./images/logo.png)

<!-- TOC insertAnchor:false -->

- [What is Athenz client sidecar](#what-is-athenz-client-sidecar)
    - [Get Athenz N-token from client sidecar](#get-athenz-n-token-from-client-sidecar)
    - [Get Athenz Access Token from client sidecar](#get-athenz-access-token-from-client-sidecar)
    - [Get Athenz Role Token from client sidecar](#get-athenz-role-token-from-client-sidecar)
    - [Proxy HTTP request (add corresponding Athenz authorization token)](#proxy-http-request-add-corresponding-athenz-authorization-token)
- [Use Case](#use-case)
- [Specification](#specification)
    - [Get N-token from Athenz through client sidecar](#get-n-token-from-athenz-through-client-sidecar)
    - [Get access token from Athenz through client sidecar](#get-access-token-from-athenz-through-client-sidecar)
    - [Get role token from Athenz through client sidecar](#get-role-token-from-athenz-through-client-sidecar)
    - [Get service certificate from Athenz through client sidecar](#get-service-certificate-from-athenz-through-client-sidecar)
    - [Proxy requests and append N-token authentication header](#proxy-requests-and-append-n-token-authentication-header)
    - [Proxy requests and append role token authentication header](#proxy-requests-and-append-role-token-authentication-header)
- [Configuration](#configuration)
- [Developer Guide](#developer-guide)
    - [Example code](#example-code)
        - [Get N-token from client sidecar](#get-n-token-from-client-sidecar)
        - [Get access token from client sidecar](#get-access-token-from-client-sidecar)
        - [Get role token from client sidecar](#get-role-token-from-client-sidecar)
        - [Get service certificate from client sidecar](#get-service-certificate-from-client-sidecar)
        - [Proxy request through client sidecar (append N-token)](#proxy-request-through-client-sidecar-append-n-token)
        - [Proxy request through client sidecar (append role token)](#proxy-request-through-client-sidecar-append-role-token)
- [Deployment Procedure](#deployment-procedure)
- [License](#license)
- [Contributor License Agreement](#contributor-license-agreement)
- [About releases](#about-releases)
- [Authors](#authors)

<!-- /TOC -->

## What is Athenz client sidecar

Athenz client sidecar is an implementation of [Kubernetes sidecar container](https://kubernetes.io/blog/2015/06/the-distributed-system-toolkit-patterns/) to provide a common interface to retrieve authentication and authorization credential from Athenz server.

### Get Athenz N-token from client sidecar

![Sidecar architecture (get N-token)](./docs/assets/client_sidecar_arch_n_token.png)

Whenever user wants to get the N-token, user does not need to focus on extra logic to generate token, user can access client sidecar container instead of implementing the logic themselves, to avoid the extra logic implemented by user.
For instance, the client sidecar container caches the token and periodically generates the token automatically. For user this logic is transparent, but it improves the overall performance as it does not generate the token every time whenever the user asks for it.

### Get Athenz Access Token from client sidecar

![Sidecar architecture (get Access token)](./docs/assets/client_sidecar_arch_access_token.png)

User can get the access token from the client sidecar container. Whenever user requests for the access token, the sidecar process will get the access token from Athenz if it is not in the cache, and cache it in memory. The background thread will update corresponding access token periodically.

### Get Athenz Role Token from client sidecar

![Sidecar architecture (get Role token)](./docs/assets/client_sidecar_arch_z_token.png)

User can get the role token from the client sidecar container. Whenever user requests for the role token, the sidecar process will get the role token from Athenz if it is not in the cache, and cache it in memory. The background thread will update corresponding role token periodically.

### Proxy HTTP request (add corresponding Athenz authorization token)

![Sidecar architecture (proxy request)](./docs/assets/client_sidecar_arch_proxy.png)

User can also use the reverse proxy endpoint to proxy the request to another server that supports Athenz token validation. The proxy endpoint will append the necessary authorization (N-token or role token) HTTP header to the request and proxy the request to the destination server. User does not need to care about the token generation logic where this sidecar container will handle it, also it supports similar caching mechanism with the N-token usage.

---

## Use Case

1. `GET /ntoken`
    - Get service token from Athenz
1. `POST /accesstoken`
    - Get access token from Athenz
1. `POST /roletoken`
    - Get role token from Athenz
1. `GET /svccert`
   - Get service certificate from Athenz
1. `/proxy/ntoken`
    - Append service token to the request header, and send the request to proxy destination
1. `/proxy/roletoken`
    - Append role token to the request header, and send the request to proxy destination

---

## Specification

### Get N-token from Athenz through client sidecar

- Only Accept HTTP GET request.
- Response body contains below information in JSON format.

| Name  | Description           | Example                                                                                             |
| ----- | --------------------- | --------------------------------------------------------------------------------------------------- |
| token | The N-token generated | v=S1;d=client;n=service;h=localhost;a=6996e6fc49915494;t=1486004464;e=1486008064;k=0;s=\[signature] |

Example:

```json
{
  "token": "v=S1;d=client;n=service;h=localhost;a=6996e6fc49915494;t=1486004464;e=1486008064;k=0;s=[signature]"
}
```

### Get access token from Athenz through client sidecar

- Only accept HTTP POST request.
- Request body must contains below information in JSON format.

| Name                | Description                                   | Required? | Example           |
| ------------------- | --------------------------------------------- | --------- | ----------------- |
| domain              | Access token domain name                      | Yes       | domain.shopping   |
| role                | Access token role name (comma separated list) | No        | user              |
| proxy_for_principal | Access token proxyForPrincipal name           | No        | proxyForPrincipal |
| expiry              | Access token expiry time (in second)          | No        | 1000              |

Example:

```json
{
  "domain": "domain.shopping",
  "role": "user",
  "proxy_for_principal": "proxyForPrincipal",
  "expiry": 1000
}
```

- Response body contains below information in JSON format.

| Name | Description | Example |
| ---- | ----------- | ------- |
| access_token | Access token | eyJraWQiOiIwIiwidHlwIjoiYXQrand0IiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiJkb21haW4udHJhdmVsLnRyYXZlbC1zaXRlIiwiaWF0IjoxNTgzNzE0NzA0LCJleHAiOjE1ODM3MTY1MDQsImlzcyI6Imh0dHBzOi8venRzLmF0aGVuei5pbyIsImF1ZCI6ImRvbWFpbi5zaG9wcGluZyIsImF1dGhfdGltZSI6MTU4MzcxNDcwNCwidmVyIjoxLCJzY3AiOlsidXNlcnMiXSwidWlkIjoiZG9tYWluLnRyYXZlbC50cmF2ZWwtc2l0ZSIsImNsaWVudF9pZCI6ImRvbWFpbi50cmF2ZWwudHJhdmVsLXNpdGUifQ.\[signature] |
| token_type   | Access token token type | Bearer |
| expires_in   | Access token expiry time (in second) | 1000 |
| scope        | Access token scope (Only added if role is not specified, space separated) | domain.shopping:role.user |

Example:

```json
{
  "access_token": "eyJraWQiOiIwIiwidHlwIjoiYXQrand0IiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiJkb21haW4udHJhdmVsLnRyYXZlbC1zaXRlIiwiaWF0IjoxNTgzNzE0NzA0LCJleHAiOjE1ODM3MTY1MDQsImlzcyI6Imh0dHBzOi8venRzLmF0aGVuei5pbyIsImF1ZCI6ImRvbWFpbi5zaG9wcGluZyIsImF1dGhfdGltZSI6MTU4MzcxNDcwNCwidmVyIjoxLCJzY3AiOlsidXNlcnMiXSwidWlkIjoiZG9tYWluLnRyYXZlbC50cmF2ZWwtc2l0ZSIsImNsaWVudF9pZCI6ImRvbWFpbi50cmF2ZWwudHJhdmVsLXNpdGUifQ.F2x9_Q4GRmgRAXB0_tQRAWSwfJ9W3VtIoIVP1F4R19Ah8x1ml8jbxe88auOGmdElR8Gd2oQBNGMSyTkBgVBi9lRmYRpvYI94DN27zy5ZQzAPx_GgWshCbv8ebK9mHmcHkvGjJQzvoc7mgtKSRCZB4fC8-95c8Nb3BlebXWOz9evhO-xlkt5QYcavvSBzU6gNzZ7IjANTwIh4_iES-drWZOZ_yg4WS9wMpk1ycJRsdr5En5QMwQJEzcMRL-5-D8gLChXEESFSsY86ekd-fXOncP1N-V1xjfVURw_TzWKiIj6DFwRsMV1dTm9ffZC0tFKOKe9M3sUYdfkm0qWuEqLjfA",
  "token_type": "Bearer",
  "expires_in": 1000,
  "scope": "domain.shopping:role.user"
}
```

### Get role token from Athenz through client sidecar

- Only accept HTTP POST request.
- Request body must contains below information in JSON format.

| Name                | Description                                 | Required? | Example           |
| ------------------- | ------------------------------------------- | --------- | ----------------- |
| domain              | Role token domain name                      | Yes       | domain.shopping   |
| role                | Role token role name (comma separated list) | No        | users             |
| proxy_for_principal | Role token proxyForPrincipal name           | No        | proxyForPrincipal |
| min_expiry          | Role token minimal expiry time (in second)  | No        | 100               |
| max_expiry          | Role token maximum expiry time (in second)  | No        | 1000              |

Example:

```json
{
  "domain": "domain.shopping",
  "role": "users",
  "proxy_for_principal": "proxyForPrincipal",
  "min_expiry": 100,
  "max_expiry": 1000
}
```

- Response body contains below information in JSON format.

| Name       | Description                              | Example |
| ---------- | ---------------------------------------- | ------- |
| token      | Role token                               | v=Z1;d=domain.shopping;r=users;p=domain.travel.travel-site;h=athenz.co.jp;a=9109ee08b79e6b63;t=1528853625;e=1528860825;k=0;i=192.168.1.1;s=\[signature] |
| expiryTime | Role token expiry time (unix timestamp)  | 1528860825 |

Example:

```json
{
  "token": "v=Z1;d=domain.shopping;r=users;p=domain.travel.travel-site;h=athenz.co.jp;a=9109ee08b79e6b63;t=1528853625;e=1528860825;k=0;i=192.168.1.1;s=s9WwmhDeO_En3dvAKvh7OKoUserfqJ0LT5Pct5Gfw5lKNKGH4vgsHLI1t0JFSQJWA1ij9ay_vWw1eKaiESfNJQOKPjAANdFZlcXqCCRUCuyAKlbX6KmWtQ9JaKSkCS8a6ReOuAmCToSqHf3STdKYF2tv1ZN17ic4se4VmT5aTig-",
  "expiryTime": 1528860825
}
```

### Get service certificate from Athenz through client sidecar

- Only Accept HTTP GET request.
- Response body contains below information in JSON format.

| Name | Description         | Example |
| ---- | ------------------- | ------- |
| cert | Service certificate | `<certificate in PEM format>` |

Example:

```json
{
  "cert": "<certificate in PEM format>"
}
```

### Proxy requests and append N-token authentication header

- Accept any HTTP request.
- Athenz client sidecar will proxy the request and append the N-token to the request header.
- The destination server will return back to user via proxy.

### Proxy requests and append role token authentication header

- Accept any HTTP request.
- Request header must contains below information.

| Name                   | Description                                                  | Required? | Example  |
| ---------------------- | ------------------------------------------------------------ | --------- | -------- |
| Athenz-Role            | The user role name used to generate the role token           | Yes       | users    |
| Athenz-Domain          | The domain name used to generate the role token              | Yes       | provider |
| Athenz-Proxy-Principal | The proxy for principal name used to generate the role token | Yes       | username |

HTTP header Example:

```none
Athenz-Role: users
Athenz-Domain: provider
Athenz-Proxy-Principal: username
```

- The destination server will return back to user via proxy.

## Configuration

- [config.go](./config/config.go)

## Developer Guide

After injecting client sidecar to user application, user application can access the client sidecar to get authorization and authentication credential from Athenz server. The client sidecar can only access by the user application injected, other application cannot access to the client sidecar. User can access client sidecar by using HTTP request.

### Example code

#### Get N-token from client sidecar

```go
import (
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/AthenZ/athenz-client-sidecar/v2/model"
)

const scURL = "127.0.0.1" // sidecar URL
const scPort = "8081"

type NTokenResponse = model.NTokenResponse

func GetNToken() (*NTokenResponse, error) {
    url := fmt.Sprintf("http://%s:%s/ntoken", scURL, scPort)

    // make request
    res, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    // validate response
    if res.StatusCode != http.StatusOK {
        err = fmt.Errorf("%s returned status code %d", url, res.StatusCode)
        return nil, err
    }

    // decode request
    var data NTokenResponse
    err = json.NewDecoder(res.Body).Decode(&data)
    if err != nil {
        return nil, err
    }

    return &data, nil
}
```

#### Get access token from client sidecar

```go
import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/AthenZ/athenz-client-sidecar/v2/model"
)

const scURL = "127.0.0.1" // sidecar URL
const scPort = "8081"

type AccessRequest = model.AccessRequest
type AccessResponse = model.AccessResponse

func GetAccessToken(domain, role, proxyForPrincipal string, expiry int64) (*AccessResponse, error) {
    url := fmt.Sprintf("http://%s:%s/accesstoken", scURL, scPort)

    r := &AccessRequest{
        Domain:            domain,
        Role:              role,
        ProxyForPrincipal: proxyForPrincipal,
        Expiry:            expiry,
    }
    reqJSON, _ := json.Marshal(r)

    // create POST request
    req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(reqJSON))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")

    // make request
    res, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    // validate response
    if res.StatusCode != http.StatusOK {
        err = fmt.Errorf("%s returned status code %d", url, res.StatusCode)
        return nil, err
    }

    // decode request
    var data AccessResponse
    err = json.NewDecoder(res.Body).Decode(&data)
    if err != nil {
        return nil, err
    }

    return &data, nil
}
```

#### Get role token from client sidecar

```go
import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/AthenZ/athenz-client-sidecar/v2/model"
)

const scURL = "127.0.0.1" // sidecar URL
const scPort = "8081"

type RoleRequest = model.RoleRequest
type RoleResponse = model.RoleResponse

func GetRoleToken(domain, role, proxyForPrincipal string, minExpiry, maxExpiry int64) (*RoleResponse, error) {
    url := fmt.Sprintf("http://%s:%s/roletoken", scURL, scPort)

    r := &RoleRequest{
        Domain:            domain,
        Role:              role,
        ProxyForPrincipal: proxyForPrincipal,
        MinExpiry:         minExpiry,
        MaxExpiry:         maxExpiry,
    }
    reqJSON, _ := json.Marshal(r)

    // create POST request
    req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(reqJSON))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")

    // make request
    res, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    // validate response
    if res.StatusCode != http.StatusOK {
        err = fmt.Errorf("%s returned status code %d", url, res.StatusCode)
        return nil, err
    }

    // decode request
    var data RoleResponse
    err = json.NewDecoder(res.Body).Decode(&data)
    if err != nil {
        return nil, err
    }

    return &data, nil
}
```

#### Get service certificate from client sidecar

```go
import (
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/AthenZ/athenz-client-sidecar/v2/model"
)

const scURL = "127.0.0.1" // sidecar URL
const scPort = "8081"

type SvcCertResponse = model.SvcCertResponse

func GetSvcCert() (*SvcCertResponse, error) {
    url := fmt.Sprintf("http://%s:%s/svccert", scURL, scPort)

    // make request
    res, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    // validate response
    if res.StatusCode != http.StatusOK {
        err = fmt.Errorf("%s returned status code %d", url, res.StatusCode)
        return nil, err
    }

    // decode request
    var data SvcCertResponse
    err = json.NewDecoder(res.Body).Decode(&data)
    if err != nil {
        return nil, err
    }

    return &data, nil
}
```

#### Proxy request through client sidecar (append N-token)

```go
const (
    scURL  = "127.0.0.1" // sidecar URL
    scPort = "8081"
)

var (
    httpClient *http.Client // the HTTP client that use the proxy to append N-token header

    // proxy URL
    proxyNTokenURL    = fmt.Sprintf("http://%s:%s/proxy/ntoken", scURL, scPort)
)

func initHTTPClient() error {
    proxyURL, err := url.Parse(proxyNTokenURL)
    if err != nil {
        return err
    }

    // transport that use the proxy, and append to the client
    transport := &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    }
    httpClient = &http.Client{
        Transport: transport,
    }

    return nil
}

func MakeRequestUsingProxy(method, targetURL string, body io.Reader) (*[]byte, error) {
    // create POST request
    req, err := http.NewRequest(method, targetURL, body)
    if err != nil {
        return nil, err
    }

    // make request through the proxy
    res, err := httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    // validate response
    if res.StatusCode != http.StatusOK {
        err = fmt.Errorf("%s returned status code %d", targetURL, res.StatusCode)
        return nil, err
    }

    // process response
    data, err := ioutil.ReadAll(res.Body)
    if err != nil {
        return nil, err
    }

    return &data, nil
}
```

#### Proxy request through client sidecar (append role token)

```go
const (
    scURL  = "127.0.0.1" // sidecar URL
    scPort = "8081"
)

var (
    httpClient *http.Client // the HTTP client that use the proxy to append role token header

    // proxy URL
    proxyRoleTokenURL = fmt.Sprintf("http://%s:%s/proxy/roletoken", scURL, scPort)
)

func initHTTPClient() error {
    proxyURL, err := url.Parse(proxyRoleTokenURL)
    if err != nil {
        return err
    }

    // transport that use the proxy, and append to the client
    transport := &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    }
    httpClient = &http.Client{
        Transport: transport,
    }

    return nil
}

func MakeRequestUsingProxy(method, targetURL string, body io.Reader, role, domain, proxyPrincipal string) (*[]byte, error) {
    // create POST request
    req, err := http.NewRequest(method, targetURL, body)
    if err != nil {
        return nil, err
    }

    // append header for the proxy
    req.Header.Set("Athenz-Role", role)
    req.Header.Set("Athenz-Domain", domain)
    req.Header.Set("Athenz-Proxy-Principal", proxyPrincipal)

    // make request through the proxy
    res, err := httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer res.Body.Close()

    // validate response
    if res.StatusCode != http.StatusOK {
        err = fmt.Errorf("%s returned status code %d", targetURL, res.StatusCode)
        return nil, err
    }

    // process response
    data, err := ioutil.ReadAll(res.Body)
    if err != nil {
        return nil, err
    }

    return &data, nil
}
```

We only provided golang example, but user can implement a client using any other language and connect to sidecar container using HTTP request.

## Deployment Procedure

1. Inject client sidecar to your K8s deployment file.

2. Deploy to K8s.

   ```bash
   kubectl apply -f injected_deployments.yaml
   ```

3. Verify if the application running

   ```bash
   # list all the pods
   kubectl get pods -n <namespace>
   # if you are not sure which namespace your application deployed, use `--all-namespaces` option
   kubectl get pods --all-namespaces

   # describe the pod to show detail information
   kubectl describe pods <pod_name>

   # check application logs
   kubectl logs <pod_name> -c <container_name>
   # e.g. to show client sidecar logs
   kubectl logs nginx-deployment-6cc8764f9c-5c6hm -c athenz-client-sidecar
   ```

## License

```markdown
Copyright (C) 2018 Yahoo Japan Corporation Athenz team.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

## Contributor License Agreement

This project requires contributors to agree to a [Contributor License Agreement (CLA)](https://gist.github.com/ydnjp/3095832f100d5c3d2592).

Note that only for contributions to the `athenz-client-sidecar` repository on the [GitHub](https://github.com/AthenZ/athenz-client-sidecar), the contributors of them shall be deemed to have agreed to the CLA without individual written agreements.

## About releases

- Releases
    - [![GitHub release (latest by date)](https://img.shields.io/github/v/release/AthenZ/athenz-client-sidecar?style=flat-square&label=Github%20version)](https://github.com/AthenZ/athenz-client-sidecar/releases/latest)
    - [![Docker Image Version (tag latest)](https://img.shields.io/docker/v/athenz/athenz-client-sidecar/latest?style=flat-square&label=Docker%20version)](https://hub.docker.com/r/athenz/athenz-client-sidecar/tags)

## Authors

- [kpango](https://github.com/kpango)
- [kevindiu](https://github.com/kevindiu)
- [TakuyaMatsu](https://github.com/TakuyaMatsu)
- [tatyano](https://github.com/tatyano)
- [WindzCUHK](https://github.com/WindzCUHK)

