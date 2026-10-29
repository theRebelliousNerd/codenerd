package articulation

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Three places describe the Piggyback envelope: the JSON Schema the model is
// held to (PiggybackEnvelopeSchema, sent to schema-capable clients as a
// response format), the struct the response is decoded into, and the key set
// strict decoding accepts (schemaAllowedKeys). Nothing but this test keeps
// them together. A field the schema asks for that the struct does not decode
// is dropped without a word; a struct field strict mode does not list turns a
// correct envelope into a parse failure.

// structKeys maps each JSON path under t to the keys its struct declares.
func structKeys(t reflect.Type, path string, out map[string][]string) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	var keys []string
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		keys = append(keys, name)
		ft := f.Type
		for ft.Kind() == reflect.Pointer || ft.Kind() == reflect.Slice {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			structKeys(ft, path+"."+name, out)
		}
	}
	sort.Strings(keys)
	out[path] = keys
}

// schemaKeys maps each JSON path under node to the property names it declares
// (following array items). Objects with free-form content (no properties) are
// leaves.
func schemaKeys(node map[string]any, path string, out map[string][]string) {
	if items, ok := node["items"].(map[string]any); ok {
		schemaKeys(items, path, out)
		return
	}
	props, ok := node["properties"].(map[string]any)
	if !ok {
		return
	}
	var keys []string
	for name, sub := range props {
		keys = append(keys, name)
		if m, ok := sub.(map[string]any); ok {
			schemaKeys(m, path+"."+name, out)
		}
	}
	sort.Strings(keys)
	out[path] = keys
}

// allowedKeys maps each JSON path under m to the keys strict decoding accepts.
func allowedKeys(m map[string]any, path string, out map[string][]string) {
	var keys []string
	for name, sub := range m {
		keys = append(keys, name)
		if subMap, ok := sub.(map[string]any); ok {
			allowedKeys(subMap, path+"."+name, out)
		}
	}
	sort.Strings(keys)
	out[path] = keys
}

func TestEnvelopeShape_SchemaStructAndStrictKeysAgree(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(PiggybackEnvelopeSchema), &schema); err != nil {
		t.Fatalf("PiggybackEnvelopeSchema is not JSON: %v", err)
	}
	fromSchema := map[string][]string{}
	schemaKeys(schema, "$", fromSchema)
	fromStruct := map[string][]string{}
	structKeys(reflect.TypeFor[PiggybackEnvelope](), "$", fromStruct)
	fromStrict := map[string][]string{}
	allowedKeys(schemaAllowedKeys, "$", fromStrict)

	// tool_args is free-form by design: the tool's own arguments.
	delete(fromStruct, "$.control_packet.tool_requests.tool_args")

	paths := map[string]struct{}{}
	for _, m := range []map[string][]string{fromSchema, fromStruct, fromStrict} {
		for p := range m {
			paths[p] = struct{}{}
		}
	}
	for p := range paths {
		s, d, k := fromSchema[p], fromStruct[p], fromStrict[p]
		if !reflect.DeepEqual(s, d) {
			t.Errorf("%s: the schema asks for %v, the struct decodes %v", p, s, d)
		}
		if !reflect.DeepEqual(d, k) {
			t.Errorf("%s: the struct decodes %v, strict decoding accepts %v", p, d, k)
		}
	}
}
