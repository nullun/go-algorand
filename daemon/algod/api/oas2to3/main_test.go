// Copyright (C) 2019-2026 Algorand Foundation Ltd.
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

package main

import (
	"encoding/json"
	"os"
	"testing"
)

// These tests guard the guarantees oas2to3 makes on top of kin-openapi's
// structural conversion. They compare the committed source (algod.oas2.json)
// against the committed generated spec (algod.oas3.yml) and fail if anything
// the code generator relies on was silently dropped. Together with the CI
// check that fails when `make generate` leaves the tree dirty, this catches a
// helper regression (e.g. after a kin-openapi upgrade, or a spec that grows a
// construct the helper does not yet handle) instead of quietly emitting wrong
// Go types.

func loadSpec(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

func collectExtensionValues(node any, key string, out map[string]bool) {
	switch n := node.(type) {
	case map[string]any:
		if v, ok := n[key].(string); ok {
			out[v] = true
		}
		for _, v := range n {
			collectExtensionValues(v, key, out)
		}
	case []any:
		for _, v := range n {
			collectExtensionValues(v, key, out)
		}
	}
}

// TestVendorExtensionsPreserved verifies that every vendor extension value in
// the source appears in the generated spec. x-go-type in particular drives the
// Go types emitted by oapi-codegen (basics.Round, uint64, ...); losing one
// silently degrades a field to a plain int.
func TestVendorExtensionsPreserved(t *testing.T) {
	src := loadSpec(t, "../algod.oas2.json")
	gen := loadSpec(t, "../algod.oas3.yml")
	// x-example is not in this list: its only use is on a security
	// definition, where the swagger.io converter dropped it (and oas2to3
	// matches that).
	for _, key := range []string{"x-go-type", "x-go-name", "x-algorand-format"} {
		srcVals := map[string]bool{}
		collectExtensionValues(src, key, srcVals)
		genVals := map[string]bool{}
		collectExtensionValues(gen, key, genVals)
		for v := range srcVals {
			if !genVals[v] {
				t.Errorf("%s value %q is in algod.oas2.json but missing from algod.oas3.yml (did the conversion drop it?)", key, v)
			}
		}
	}
}

// TestProducesMediaTypesPreserved verifies that every operation declaring
// extra response media types via Swagger 2.0 "produces" (e.g.
// application/msgpack) keeps content entries for all of them in the generated
// spec. kin-openapi leaves shared responses as $refs that only carry
// application/json, which silently dropped msgpack from every
// msgpack-capable endpoint until oas2to3 started inlining and expanding them.
func TestProducesMediaTypesPreserved(t *testing.T) {
	src := loadSpec(t, "../algod.oas2.json")
	gen := loadSpec(t, "../algod.oas3.yml")

	srcPaths, _ := src["paths"].(map[string]any)
	genPaths, _ := gen["paths"].(map[string]any)
	checked := 0
	for path, pi := range srcPaths {
		item, ok := pi.(map[string]any)
		if !ok {
			continue
		}
		for method, o := range item {
			op, ok := o.(map[string]any)
			if !ok {
				continue
			}
			produces, ok := op["produces"].([]any)
			if !ok || len(produces) < 2 {
				continue
			}
			genItem, _ := genPaths[path].(map[string]any)
			genOp, _ := genItem[method].(map[string]any)
			responses, _ := genOp["responses"].(map[string]any)
			if responses == nil {
				t.Errorf("%s %s: operation missing from generated spec", method, path)
				continue
			}
			for code, r := range responses {
				resp, ok := r.(map[string]any)
				if !ok {
					continue
				}
				content, ok := resp["content"].(map[string]any)
				if !ok || len(content) == 0 {
					continue // schema-less response
				}
				for _, mt := range produces {
					name, _ := mt.(string)
					if _, ok := content[name]; !ok {
						t.Errorf("%s %s response %s: media type %q from produces is missing", method, path, code, name)
					}
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Skip("source declares no operations with multiple produces media types")
	}
}

// TestByteStringsHaveBase64Pattern verifies every "format: byte" string
// carries the base64 pattern the swagger.io converter used to attach --
// except inside request bodies, which that converter did not decorate.
func TestByteStringsHaveBase64Pattern(t *testing.T) {
	gen := loadSpec(t, "../algod.oas3.yml")
	var missing int
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if s, _ := n["type"].(string); s == "string" {
				if f, _ := n["format"].(string); f == "byte" {
					if p, _ := n["pattern"].(string); p != base64Pattern {
						missing++
					}
				}
			}
			for k, v := range n {
				if k == "requestBody" {
					continue
				}
				walk(v)
			}
		case []any:
			for _, v := range n {
				walk(v)
			}
		}
	}
	walk(gen)
	if missing > 0 {
		t.Errorf("%d \"format: byte\" schema(s) are missing the base64 pattern", missing)
	}
}

// TestCollectionFormatsMapped verifies that every array parameter which declared
// a Swagger 2.0 collectionFormat in the source has an explicit style/explode in
// the generated spec (otherwise oapi-codegen would assume the OpenAPI 3 default
// explode=true and parse "a,b" as repeated params).
func TestCollectionFormatsMapped(t *testing.T) {
	src := loadSpec(t, "../algod.oas2.json")
	gen := loadSpec(t, "../algod.oas3.yml")

	want := map[string]bool{}
	var scan func(node any)
	scan = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if typ, _ := n["type"].(string); typ == "array" {
				if _, ok := n["collectionFormat"].(string); ok {
					if name, _ := n["name"].(string); name != "" {
						want[name] = true
					}
				}
			}
			for _, v := range n {
				scan(v)
			}
		case []any:
			for _, v := range n {
				scan(v)
			}
		}
	}
	scan(src)
	if len(want) == 0 {
		t.Skip("source declares no array parameters with a collectionFormat")
	}

	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if name, _ := n["name"].(string); want[name] {
				if in, _ := n["in"].(string); in == "query" || in == "cookie" {
					_, hasStyle := n["style"]
					_, hasExplode := n["explode"]
					if !hasStyle || !hasExplode {
						t.Errorf("array query param %q lost its collectionFormat style/explode mapping", name)
					}
				}
			}
			for _, v := range n {
				walk(v)
			}
		case []any:
			for _, v := range n {
				walk(v)
			}
		}
	}
	walk(gen)
}
