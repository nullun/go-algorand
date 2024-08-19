// Package data provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/algorand/oapi-codegen DO NOT EDIT.
package data

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	. "github.com/algorand/go-algorand/daemon/algod/api/server/v2/generated/model"
	"github.com/algorand/oapi-codegen/pkg/runtime"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
)

// ServerInterface represents all server handlers.
type ServerInterface interface {
	// Removes minimum sync round restriction from the ledger.
	// (DELETE /v2/ledger/sync)
	UnsetSyncRound(ctx echo.Context) error
	// Returns the minimum sync round the ledger is keeping in cache.
	// (GET /v2/ledger/sync)
	GetSyncRound(ctx echo.Context) error
	// Given a round, tells the ledger to keep that round in its cache.
	// (POST /v2/ledger/sync/{round})
	SetSyncRound(ctx echo.Context, round uint64) error
}

// ServerInterfaceWrapper converts echo contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler ServerInterface
}

// UnsetSyncRound converts echo context to params.
func (w *ServerInterfaceWrapper) UnsetSyncRound(ctx echo.Context) error {
	var err error

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.UnsetSyncRound(ctx)
	return err
}

// GetSyncRound converts echo context to params.
func (w *ServerInterfaceWrapper) GetSyncRound(ctx echo.Context) error {
	var err error

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetSyncRound(ctx)
	return err
}

// SetSyncRound converts echo context to params.
func (w *ServerInterfaceWrapper) SetSyncRound(ctx echo.Context) error {
	var err error
	// ------------- Path parameter "round" -------------
	var round uint64

	err = runtime.BindStyledParameterWithLocation("simple", false, "round", runtime.ParamLocationPath, ctx.Param("round"), &round)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter round: %s", err))
	}

	ctx.Set(Api_keyScopes, []string{""})

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.SetSyncRound(ctx, round)
	return err
}

// This is a simple interface which specifies echo.Route addition functions which
// are present on both echo.Echo and echo.Group, since we want to allow using
// either of them for path registration
type EchoRouter interface {
	CONNECT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	DELETE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	HEAD(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	OPTIONS(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PATCH(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	POST(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	PUT(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
	TRACE(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) *echo.Route
}

// RegisterHandlers adds each server route to the EchoRouter.
func RegisterHandlers(router EchoRouter, si ServerInterface, m ...echo.MiddlewareFunc) {
	RegisterHandlersWithBaseURL(router, si, "", m...)
}

// Registers handlers, and prepends BaseURL to the paths, so that the paths
// can be served under a prefix.
func RegisterHandlersWithBaseURL(router EchoRouter, si ServerInterface, baseURL string, m ...echo.MiddlewareFunc) {

	wrapper := ServerInterfaceWrapper{
		Handler: si,
	}

	router.DELETE(baseURL+"/v2/ledger/sync", wrapper.UnsetSyncRound, m...)
	router.GET(baseURL+"/v2/ledger/sync", wrapper.GetSyncRound, m...)
	router.POST(baseURL+"/v2/ledger/sync/:round", wrapper.SetSyncRound, m...)

}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
}

// GetSwagger returns the content of the embedded swagger specification file
// or error if failed to decode
func decodeSpec() ([]byte, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %s", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}

	return buf.Bytes(), nil
}

var rawSpec = decodeSpecCached()

// a naive cached of a decoded swagger spec
func decodeSpecCached() func() ([]byte, error) {
	data, err := decodeSpec()
	return func() ([]byte, error) {
		return data, err
	}
}

// Constructs a synthetic filesystem for resolving external references when loading openapi specifications.
func PathToRawSpec(pathToFile string) map[string]func() ([]byte, error) {
	var res = make(map[string]func() ([]byte, error))
	if len(pathToFile) > 0 {
		res[pathToFile] = rawSpec
	}

	return res
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file. The external references of Swagger specification are resolved.
// The logic of resolving external references is tightly connected to "import-mapping" feature.
// Externally referenced files must be embedded in the corresponding golang packages.
// Urls can be supported but this task was out of the scope.
func GetSwagger() (swagger *openapi3.T, err error) {
	var resolvePath = PathToRawSpec("")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(loader *openapi3.Loader, url *url.URL) ([]byte, error) {
		var pathToFile = url.String()
		pathToFile = path.Clean(pathToFile)
		getSpec, ok := resolvePath[pathToFile]
		if !ok {
			err1 := fmt.Errorf("path not found: %s", pathToFile)
			return nil, err1
		}
		return getSpec()
	}
	var specData []byte
	specData, err = rawSpec()
	if err != nil {
		return
	}
	swagger, err = loader.LoadFromData(specData)
	if err != nil {
		return
	}
	return
}
