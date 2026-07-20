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

// Command oas2to3 converts an OpenAPI 2 (Swagger) document read from stdin
// into an OpenAPI 3 document written to stdout. It replaces the previous
// dependency on the external converter.swagger.io HTTP service, performing
// the conversion locally with github.com/getkin/kin-openapi (already a
// dependency of this module).
//
// kin-openapi performs the structural conversion; the rest of this command
// post-processes its output so the result is byte-identical (after
// jsoncanon.py) to what the swagger.io converter produced for this spec. That
// keeps the published algod.oas3.yml stable for every downstream consumer and
// keeps the oapi-codegen output (which relies on vendor extensions like
// x-go-type, inlined parameters, and collectionFormat-derived style/explode)
// unchanged. The differences papered over include: restoring dropped x-*
// vendor extensions, inlining parameter and response $refs into operations,
// expanding response content per the Swagger 2.0 produces list (msgpack),
// translating collectionFormat to style/explode, normalizing object schemas,
// and matching several cosmetic swagger-parser behaviors.
//
// Scope: this helper reproduces only the swagger-parser conversion behaviors
// that algod.oas2.json actually exercises. It intentionally does not implement
// conversions the spec does not use -- formData/file parameters, type:number,
// oauth2 security flows, allOf/discriminator/additionalProperties schemas,
// response headers, and the tsv collectionFormat (which has no OpenAPI 3
// equivalent). If algod.oas2.json grows any of these, extend this helper (and
// the guards in main_test.go) to match. The guards fail loudly if a
// relied-upon vendor extension, byte pattern, or collectionFormat mapping
// goes missing.
//
// The output is not canonicalized here; callers pipe it through jsoncanon.py
// to match the committed algod.oas3.yml format.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "oas2to3: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	// Structural conversion via kin-openapi.
	var doc2 openapi2.T
	if err := json.Unmarshal(in, &doc2); err != nil {
		return fmt.Errorf("parsing OpenAPI 2 document: %w", err)
	}
	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return fmt.Errorf("converting to OpenAPI 3: %w", err)
	}
	converted, err := json.Marshal(doc3)
	if err != nil {
		return fmt.Errorf("marshaling converted document: %w", err)
	}

	// Restore vendor extensions lost during the structural conversion. We work
	// on generic JSON maps rather than the typed structs so the restoration is
	// agnostic to where extensions appear in the schema tree.
	var v2, v3 map[string]any
	if err := json.Unmarshal(in, &v2); err != nil {
		return fmt.Errorf("parsing source document as JSON: %w", err)
	}
	if err := json.Unmarshal(converted, &v3); err != nil {
		return fmt.Errorf("parsing converted document as JSON: %w", err)
	}
	restoreExtensions(v2, v3)

	out, err := json.Marshal(v3)
	if err != nil {
		return fmt.Errorf("marshaling OpenAPI 3 document: %w", err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return fmt.Errorf("writing stdout: %w", err)
	}
	return nil
}

// restoreExtensions copies the vendor (x-*) extensions from the source
// OpenAPI 2 document (v2) onto the converted OpenAPI 3 document (v3).
func restoreExtensions(v2, v3 map[string]any) {
	// Named schemas: definitions -> components.schemas, matched by name. The
	// schema trees are structurally identical, so a shape-preserving merge
	// restores extensions on the schema and all of its nested properties.
	schemas := dig(v3, "components", "schemas")
	if defs, ok := v2["definitions"].(map[string]any); ok {
		for name, def := range defs {
			if s, ok := schemas[name]; ok {
				mergeExtensions(s, def)
			}
		}
	}

	// Named responses: responses -> components.responses, matched by name. In
	// v2 the schema hangs directly off the response; in v3 it lives under
	// content[mediatype].schema.
	responses := dig(v3, "components", "responses")
	if v2resp, ok := v2["responses"].(map[string]any); ok {
		for name, r := range v2resp {
			src, ok := r.(map[string]any)
			if !ok {
				continue
			}
			schema, ok := src["schema"]
			if !ok {
				continue
			}
			dst, ok := responses[name].(map[string]any)
			if !ok {
				continue
			}
			content, ok := dst["content"].(map[string]any)
			if !ok {
				continue
			}
			for _, mt := range content {
				if m, ok := mt.(map[string]any); ok {
					mergeExtensions(m["schema"], schema)
				}
			}
		}
	}

	// Operations: the swagger.io converter inlined parameter definitions into
	// each operation (while keeping the shared definitions under
	// components.parameters). kin-openapi instead leaves $ref pointers. Inline
	// them so oapi-codegen generates the same per-operation types (e.g. enum
	// types) it did before.
	inlineParameterRefs(v3)

	// The swagger.io converter also copied a path item's shared parameters
	// into each of its operations. Do the same.
	distributePathParameters(v3)

	// Responses: the swagger.io converter inlined named responses into each
	// operation and emitted one content entry per media type in the
	// operation's Swagger 2.0 "produces" list. kin-openapi leaves a $ref to
	// components.responses, which only carries application/json -- silently
	// dropping application/msgpack from every msgpack-capable endpoint.
	// Inline the refs and expand the content per produces.
	inlineResponseRefs(v2, v3)

	// Responses with no schema: the swagger.io converter emitted an empty
	// content object where kin-openapi omits the key entirely.
	addEmptyResponseContent(v3)

	// Parameters: kin-openapi keeps a parameter's extensions on the parameter
	// object but oapi-codegen reads x-go-type from the parameter's schema, so
	// push them down (this matches the swagger.io converter output). This runs
	// after inlining so the inlined copies are updated too.
	pushDownParameterExtensions(v3)

	// Array query parameters: kin-openapi drops the Swagger 2.0
	// collectionFormat, which leaves oapi-codegen to assume the OpenAPI 3
	// default of explode=true. The swagger.io converter translated
	// collectionFormat into an explicit style/explode. Restore that so
	// e.g. csv parameters keep parsing as "a,b" rather than "a&...&b".
	applyCollectionFormats(v2, v3)

	// Byte strings: the swagger.io converter decorated every "format: byte"
	// string with a base64 validation pattern (it is not present in the
	// source document), except in request bodies, which it converted through
	// a separate code path. Reproduce that so the published spec keeps the
	// same constraints. This does not affect the generated Go ([]byte either
	// way).
	applyByteStringPatterns(v3)

	// Body parameters: the swagger.io converter recorded the original body
	// parameter name as x-codegen-request-body-name on the operation.
	// kin-openapi instead sets x-originalParamName on the request body.
	// Emit the former and drop the latter.
	renameBodyParamExtension(v3)

	// Object schemas: swagger-parser's models always serialize both "type"
	// and "properties" for object schemas, so the converter added
	// "type": "object" to schemas that only declared properties and
	// "properties": {} to schemas that only declared type. kin-openapi
	// passes schemas through as written. Normalize to match.
	normalizeObjectSchemas(v3)

	// The swagger.io converter sorted every "required" list alphabetically
	// (it stores them as a set); kin-openapi preserves source order. The
	// order is not meaningful, but sort to keep the output stable.
	sortRequiredLists(v3)

	// Root-level cosmetics: match the converter's OpenAPI version stamp and
	// its x-original-swagger-version marker, drop the empty description
	// strings kin-openapi adds to named responses that have none in the
	// source, and drop extensions on security schemes (the swagger.io
	// converter did not carry them over).
	v3["openapi"] = "3.0.1"
	v3["x-original-swagger-version"] = "2.0"
	dropEmptyDescriptions(v3)
	for _, scheme := range dig(v3, "components", "securitySchemes") {
		if m, ok := scheme.(map[string]any); ok {
			for k := range m {
				if strings.HasPrefix(k, "x-") {
					delete(m, k)
				}
			}
		}
	}
}

// base64Pattern validates the base64 encoding used by "format: byte" strings.
// It matches the pattern the swagger.io converter injected for such fields.
const base64Pattern = `^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`

// applyByteStringPatterns sets base64Pattern on every "format: byte" string
// schema that does not already have a pattern, except inside request bodies
// (the swagger.io converter did not decorate body parameter schemas).
func applyByteStringPatterns(node any) {
	switch n := node.(type) {
	case map[string]any:
		t, _ := n["type"].(string)
		f, _ := n["format"].(string)
		if t == "string" && f == "byte" {
			if _, ok := n["pattern"]; !ok {
				n["pattern"] = base64Pattern
			}
		}
		for k, v := range n {
			if k == "requestBody" {
				continue
			}
			applyByteStringPatterns(v)
		}
	case []any:
		for _, v := range n {
			applyByteStringPatterns(v)
		}
	}
}

// collectionFormatStyles maps a Swagger 2.0 collectionFormat to the OpenAPI 3
// (style, explode) pair for query/cookie parameters.
var collectionFormatStyles = map[string]struct {
	style   string
	explode bool
}{
	"csv":   {"form", false},
	"multi": {"form", true},
	"ssv":   {"spaceDelimited", false},
	"pipes": {"pipeDelimited", false},
}

// applyCollectionFormats reads the collectionFormat of each array parameter in
// the source document and sets the corresponding style/explode on the matching
// parameters in the converted document. Parameters are matched by name, which
// is unambiguous in this spec (a given parameter name always uses the same
// collectionFormat).
func applyCollectionFormats(v2, v3 map[string]any) {
	formats := map[string]string{}
	var scan func(node any)
	scan = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if name, ok := n["name"].(string); ok {
				if typ, _ := n["type"].(string); typ == "array" {
					if cf, ok := n["collectionFormat"].(string); ok {
						formats[name] = cf
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
	scan(v2["parameters"])
	scan(v2["paths"])
	if len(formats) == 0 {
		return
	}

	var apply func(node any)
	apply = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if in, _ := n["in"].(string); in == "query" || in == "cookie" {
				if isArraySchema(n["schema"]) {
					if name, ok := n["name"].(string); ok {
						if sc, ok := collectionFormatStyles[formats[name]]; ok {
							if _, exists := n["style"]; !exists {
								n["style"] = sc.style
							}
							if _, exists := n["explode"]; !exists {
								n["explode"] = sc.explode
							}
						}
					}
				}
			}
			for _, v := range n {
				apply(v)
			}
		case []any:
			for _, v := range n {
				apply(v)
			}
		}
	}
	apply(v3)
}

// isArraySchema reports whether the given (parameter) schema describes an array.
func isArraySchema(schema any) bool {
	m, ok := schema.(map[string]any)
	if !ok {
		return false
	}
	t, _ := m["type"].(string)
	return t == "array"
}

// inlineParameterRefs replaces $ref pointers to components.parameters that
// appear in "parameters" arrays with a copy of the referenced definition. The
// shared definitions are left in place.
func inlineParameterRefs(v3 map[string]any) {
	params := dig(v3, "components", "parameters")
	if params == nil {
		return
	}
	const prefix = "#/components/parameters/"
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if arr, ok := n["parameters"].([]any); ok {
				for i, el := range arr {
					m, ok := el.(map[string]any)
					if !ok {
						continue
					}
					ref, ok := m["$ref"].(string)
					if !ok || !strings.HasPrefix(ref, prefix) {
						continue
					}
					if resolved, ok := params[strings.TrimPrefix(ref, prefix)]; ok {
						arr[i] = deepCopy(resolved)
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
	walk(v3)
}

// deepCopy returns an independent copy of a JSON-like value so that inlined
// parameters do not alias the shared definition they were copied from.
func deepCopy(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}

// dig walks a chain of nested object keys, returning nil if any step is missing
// or is not an object. Indexing the nil result is safe and yields (nil, false).
func dig(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

// mergeExtensions copies x-* keys present in src (but missing in dst) onto dst,
// recursing through shared keys so extensions nested inside properties, items,
// allOf, etc. are restored. src and dst are expected to be the same shape.
func mergeExtensions(dst, src any) {
	switch s := src.(type) {
	case map[string]any:
		d, ok := dst.(map[string]any)
		if !ok {
			return
		}
		for k, v := range s {
			if strings.HasPrefix(k, "x-") {
				if _, exists := d[k]; !exists {
					d[k] = v
				}
				continue
			}
			if dv, ok := d[k]; ok {
				mergeExtensions(dv, v)
			}
		}
	case []any:
		d, ok := dst.([]any)
		if !ok {
			return
		}
		for i := 0; i < len(s) && i < len(d); i++ {
			mergeExtensions(d[i], s[i])
		}
	}
}

// httpMethods are the operation keys of an OpenAPI path item.
var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// distributePathParameters copies a path item's shared "parameters" list into
// each of its operations, matching the swagger.io converter, which did not
// emit path-level parameters.
func distributePathParameters(v3 map[string]any) {
	for _, pi := range dig(v3, "paths") {
		item, ok := pi.(map[string]any)
		if !ok {
			continue
		}
		shared, ok := item["parameters"].([]any)
		if !ok {
			continue
		}
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			existing, _ := op["parameters"].([]any)
			merged := make([]any, 0, len(shared)+len(existing))
			for _, p := range shared {
				merged = append(merged, deepCopy(p))
			}
			op["parameters"] = append(merged, existing...)
		}
		delete(item, "parameters")
	}
}

// inlineResponseRefs replaces $ref pointers to components.responses in each
// operation with a copy of the referenced definition, and expands the copy's
// content to cover every media type in the operation's Swagger 2.0 "produces"
// list (the schema is the same for each). The shared definitions are left in
// place; they keep application/json only, as the converter emitted them.
func inlineResponseRefs(v2, v3 map[string]any) {
	responses := dig(v3, "components", "responses")
	if responses == nil {
		return
	}
	globalProduces, _ := v2["produces"].([]any)
	const prefix = "#/components/responses/"
	for path, pi := range dig(v3, "paths") {
		item, ok := pi.(map[string]any)
		if !ok {
			continue
		}
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			produces := globalProduces
			if v2op, ok := dig(v2, "paths", path)[method].(map[string]any); ok {
				if p, ok := v2op["produces"].([]any); ok {
					produces = p
				}
			}
			resp, ok := op["responses"].(map[string]any)
			if !ok {
				continue
			}
			for code, r := range resp {
				m, ok := r.(map[string]any)
				if !ok {
					continue
				}
				ref, ok := m["$ref"].(string)
				if !ok || !strings.HasPrefix(ref, prefix) {
					continue
				}
				resolved, ok := responses[strings.TrimPrefix(ref, prefix)].(map[string]any)
				if !ok {
					continue
				}
				inlined := deepCopy(resolved).(map[string]any)
				if jsonMedia, ok := dig(inlined, "content")["application/json"]; ok {
					content := map[string]any{}
					for _, mt := range produces {
						if name, ok := mt.(string); ok {
							content[name] = deepCopy(jsonMedia)
						}
					}
					inlined["content"] = content
				}
				resp[code] = inlined
			}
		}
	}
}

// addEmptyResponseContent gives every operation response that has no content
// an empty content object, as the swagger.io converter did.
func addEmptyResponseContent(v3 map[string]any) {
	for _, pi := range dig(v3, "paths") {
		item, ok := pi.(map[string]any)
		if !ok {
			continue
		}
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			resp, ok := op["responses"].(map[string]any)
			if !ok {
				continue
			}
			for _, r := range resp {
				if m, ok := r.(map[string]any); ok {
					if _, ok := m["content"]; !ok {
						m["content"] = map[string]any{}
					}
				}
			}
		}
	}
}

// renameBodyParamExtension moves kin-openapi's x-originalParamName (set on the
// request body) to the operation as x-codegen-request-body-name, the extension
// the swagger.io converter emitted.
func renameBodyParamExtension(v3 map[string]any) {
	for _, pi := range dig(v3, "paths") {
		item, ok := pi.(map[string]any)
		if !ok {
			continue
		}
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			body, ok := op["requestBody"].(map[string]any)
			if !ok {
				continue
			}
			if name, ok := body["x-originalParamName"]; ok {
				op["x-codegen-request-body-name"] = name
				delete(body, "x-originalParamName")
			}
		}
	}
}

// normalizeObjectSchemas reproduces how swagger-parser's models serialize
// object schemas: schemas declaring only properties gain "type": "object"
// (everywhere), and bare "type": "object" schemas gain "properties": {} --
// except at schema roots (named definitions and the direct value of a
// "schema" key), which the converter serialized without the empty map.
func normalizeObjectSchemas(v3 map[string]any) {
	for _, s := range dig(v3, "components", "schemas") {
		normalizeSchema(s, true)
	}
	for k, v := range v3 {
		if k != "components" {
			normalizeSchema(v, false)
			continue
		}
		if comps, ok := v.(map[string]any); ok {
			for ck, cv := range comps {
				if ck != "schemas" {
					normalizeSchema(cv, false)
				}
			}
		}
	}
}

// normalizeSchema is the recursive worker for normalizeObjectSchemas. root
// marks a schema root, which does not receive an empty properties map.
func normalizeSchema(node any, root bool) {
	switch n := node.(type) {
	case map[string]any:
		if props, ok := n["properties"].(map[string]any); ok {
			if _, ok := n["type"]; !ok {
				n["type"] = "object"
			}
			// Recurse into the property schemas, not the properties map
			// itself, so a property named "type" or "properties" cannot be
			// mistaken for a schema keyword.
			for _, v := range props {
				normalizeSchema(v, false)
			}
		} else if t, _ := n["type"].(string); t == "object" && !root {
			n["properties"] = map[string]any{}
		}
		for k, v := range n {
			if k == "properties" {
				continue
			}
			normalizeSchema(v, k == "schema")
		}
	case []any:
		for _, v := range n {
			normalizeSchema(v, false)
		}
	}
}

// sortRequiredLists alphabetically sorts every "required" list of property
// names, matching the swagger.io converter (which kept them as sets). The
// boolean "required" on parameters is untouched.
func sortRequiredLists(node any) {
	switch n := node.(type) {
	case map[string]any:
		if req, ok := n["required"].([]any); ok {
			names := make([]string, 0, len(req))
			ok := true
			for _, v := range req {
				s, isString := v.(string)
				if !isString {
					ok = false
					break
				}
				names = append(names, s)
			}
			if ok {
				sort.Strings(names)
				for i, s := range names {
					req[i] = s
				}
			}
		}
		for _, v := range n {
			sortRequiredLists(v)
		}
	case []any:
		for _, v := range n {
			sortRequiredLists(v)
		}
	}
}

// dropEmptyDescriptions removes the empty description strings kin-openapi adds
// to named responses whose source declares none.
func dropEmptyDescriptions(node any) {
	switch n := node.(type) {
	case map[string]any:
		if d, ok := n["description"].(string); ok && d == "" {
			delete(n, "description")
		}
		for _, v := range n {
			dropEmptyDescriptions(v)
		}
	case []any:
		for _, v := range n {
			dropEmptyDescriptions(v)
		}
	}
}

// parameterLocations are the "in" values for a real parameter (as opposed to a
// header or request body, which take different code paths in the converter).
var parameterLocations = map[string]bool{"path": true, "query": true, "header": true, "cookie": true}

// pushDownParameterExtensions walks the document and, for every parameter
// object, copies its x-* extensions onto its schema so oapi-codegen sees them.
func pushDownParameterExtensions(node any) {
	switch n := node.(type) {
	case map[string]any:
		if in, ok := n["in"].(string); ok && parameterLocations[in] {
			if schema, ok := n["schema"].(map[string]any); ok {
				for k, v := range n {
					if strings.HasPrefix(k, "x-") {
						if _, exists := schema[k]; !exists {
							schema[k] = v
						}
					}
				}
			}
		}
		for _, v := range n {
			pushDownParameterExtensions(v)
		}
	case []any:
		for _, v := range n {
			pushDownParameterExtensions(v)
		}
	}
}
