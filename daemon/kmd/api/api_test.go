// Copyright (C) 2019-2026 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/daemon/kmd/config"
	"github.com/algorand/go-algorand/daemon/kmd/session"
	"github.com/algorand/go-algorand/logging"
	"github.com/algorand/go-algorand/test/partitiontest"
)

const testToken = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func makeTestHandler(t *testing.T, origins []string, pna bool, reqCB func()) http.Handler {
	t.Helper()
	cfg := config.KMDConfig{SessionLifetimeSecs: 60}
	sm := session.MakeManager(cfg)
	t.Cleanup(func() { sm.Kill() })
	log := logging.TestingLog(t)
	if reqCB == nil {
		reqCB = func() {}
	}
	return Handler(sm, log, origins, testToken, pna, reqCB)
}

func TestVersionsEndpoint(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, false, nil)
	req := httptest.NewRequest("GET", "/versions", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusOK, rr.Code)
	a.Equal("application/json", rr.Header().Get("Content-Type"))

	var resp struct {
		Versions []string `json:"versions"`
	}
	a.NoError(json.Unmarshal(rr.Body.Bytes(), &resp))
	a.Equal([]string{"v1"}, resp.Versions)
}

func TestVersionsNoAuthRequired(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, false, nil)
	// No X-KMD-API-Token header set
	req := httptest.NewRequest("GET", "/versions", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusOK, rr.Code)
}

func TestSwaggerEndpoint(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, false, nil)
	req := httptest.NewRequest("GET", "/swagger.json", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusOK, rr.Code)
	a.Equal("application/json", rr.Header().Get("Content-Type"))
}

func TestOptionsCatchAll(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, false, nil)

	for _, path := range []string{"/", "/versions", "/v1/wallets", "/v1/key", "/nonexistent"} {
		req := httptest.NewRequest("OPTIONS", path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		a.Equal(http.StatusOK, rr.Code, "OPTIONS %s should return 200", path)
	}
}

func TestCORSAllowedOrigin(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, []string{"http://localhost:3000"}, false, nil)

	req := httptest.NewRequest("GET", "/versions", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusOK, rr.Code)
	a.Equal("http://localhost:3000", rr.Header().Get("Access-Control-Allow-Origin"))
	a.Contains(rr.Header().Get("Access-Control-Allow-Methods"), "GET")
	a.Contains(rr.Header().Get("Access-Control-Allow-Headers"), "X-KMD-API-Token")
}

func TestCORSDisallowedOrigin(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, []string{"http://localhost:3000"}, false, nil)

	req := httptest.NewRequest("GET", "/versions", nil)
	req.Header.Set("Origin", "http://evil.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusOK, rr.Code)
	a.Empty(rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSWildcard(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, []string{"*"}, false, nil)

	req := httptest.NewRequest("GET", "/versions", nil)
	req.Header.Set("Origin", "http://anything.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal("http://anything.example.com", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSNoOriginHeader(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, []string{"http://localhost:3000"}, false, nil)

	req := httptest.NewRequest("GET", "/versions", nil)
	// No Origin header
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusOK, rr.Code)
	a.Empty(rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestPNAEnabled(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, []string{"*"}, true, nil)

	req := httptest.NewRequest("OPTIONS", "/v1/wallets", nil)
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusOK, rr.Code)
	a.Equal("true", rr.Header().Get("Access-Control-Allow-Private-Network"))
}

func TestPNADisabled(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, []string{"*"}, false, nil)

	req := httptest.NewRequest("OPTIONS", "/v1/wallets", nil)
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusOK, rr.Code)
	a.Empty(rr.Header().Get("Access-Control-Allow-Private-Network"))
}

func TestPNANotSetForNonOptions(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, true, nil)

	req := httptest.NewRequest("GET", "/versions", nil)
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusOK, rr.Code)
	a.Empty(rr.Header().Get("Access-Control-Allow-Private-Network"))
}

func TestV1AuthRequired(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, false, nil)

	// Request without auth token should get 401
	req := httptest.NewRequest("GET", "/v1/wallets", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusUnauthorized, rr.Code)
}

func TestV1AuthWrongToken(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, false, nil)

	req := httptest.NewRequest("GET", "/v1/wallets", nil)
	req.Header.Set("X-KMD-API-Token", strings.Repeat("b", 64))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusUnauthorized, rr.Code)
}

func TestV1AuthValidToken(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, false, nil)

	req := httptest.NewRequest("GET", "/v1/wallets", nil)
	req.Header.Set("X-KMD-API-Token", testToken)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Should not be 401 — may be 200 or 500 depending on driver state,
	// but auth passed
	a.NotEqual(http.StatusUnauthorized, rr.Code)
}

func TestRequestCallback(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	var callCount atomic.Int32
	h := makeTestHandler(t, nil, false, func() {
		callCount.Add(1)
	})

	// Authenticated v1 request should trigger the callback
	req := httptest.NewRequest("GET", "/v1/wallets", nil)
	req.Header.Set("X-KMD-API-Token", testToken)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(int32(1), callCount.Load())
}

func TestRequestCallbackNotCalledWithoutAuth(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	var callCount atomic.Int32
	h := makeTestHandler(t, nil, false, func() {
		callCount.Add(1)
	})

	// Unauthenticated request — auth middleware rejects before reqCB runs
	req := httptest.NewRequest("GET", "/v1/wallets", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	a.Equal(http.StatusUnauthorized, rr.Code)
	a.Equal(int32(0), callCount.Load())
}

func TestRouteMethodEnforcement(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, false, nil)

	tests := []struct {
		method string
		path   string
		want   int // expected status (405 for wrong method)
	}{
		// Correct methods should not 404 or 405
		{"GET", "/v1/wallets", http.StatusUnauthorized}, // auth blocks but route matches
		// Wrong methods should 405
		{"POST", "/v1/wallets", http.StatusMethodNotAllowed},
		{"DELETE", "/v1/wallets", http.StatusMethodNotAllowed},
		{"GET", "/v1/wallet", http.StatusMethodNotAllowed},
		{"GET", "/v1/key", http.StatusMethodNotAllowed},
		// Non-existent paths get 405 because the OPTIONS catch-all means
		// every path has at least one method handler
		{"GET", "/nonexistent", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		a.Equal(tt.want, rr.Code, "%s %s", tt.method, tt.path)
	}
}

func TestAllV1RoutesReachable(t *testing.T) {
	partitiontest.PartitionTest(t)
	t.Parallel()
	a := require.New(t)

	h := makeTestHandler(t, nil, false, nil)

	// Every registered v1 route should return 401 (auth needed), not 404
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/wallets"},
		{"POST", "/v1/wallet"},
		{"POST", "/v1/wallet/init"},
		{"POST", "/v1/wallet/release"},
		{"POST", "/v1/wallet/renew"},
		{"POST", "/v1/wallet/rename"},
		{"POST", "/v1/wallet/info"},
		{"POST", "/v1/master-key/export"},
		{"POST", "/v1/key/list"},
		{"POST", "/v1/key/import"},
		{"POST", "/v1/key/export"},
		{"POST", "/v1/key"},
		{"DELETE", "/v1/key"},
		{"POST", "/v1/multisig/list"},
		{"POST", "/v1/multisig/sign"},
		{"POST", "/v1/multisig/signprogram"},
		{"POST", "/v1/multisig/import"},
		{"POST", "/v1/multisig/export"},
		{"DELETE", "/v1/multisig"},
		{"POST", "/v1/transaction/sign"},
		{"POST", "/v1/program/sign"},
	}

	for _, r := range routes {
		req := httptest.NewRequest(r.method, r.path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		// Without auth token we expect 401, proving the route exists and
		// reached the auth middleware (not a 404)
		a.Equal(http.StatusUnauthorized, rr.Code, "%s %s should be reachable (got %d)", r.method, r.path, rr.Code)
	}
}
