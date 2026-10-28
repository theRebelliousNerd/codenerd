package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// FullJSON renders the config with every configurable field present and the
// value in force: raw (the user's file) laid over the defaults, encoded without
// omitempty, so a false, a 0 and an "" are written down like anything else.
// Field order is the schema's. Sections the schema leaves nil are written as
// their zero struct, so their keys are visible too.
//
// It returns a document to merge by hand; nothing here writes config.json.
// Steve, 2026-09-21: "i want literally every single thing that can be
// configured displayed in that json file and we need to configure it all by
// hand and not just default to the defaults".
func FullJSON(raw []byte) ([]byte, error) {
	cfg := DefaultUserConfig()
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := rejectRemovedKeys(raw); err != nil {
			return nil, err
		}
		if err := decodeStrictJSON(raw, cfg); err != nil {
			return nil, err
		}
	}
	// Reasoning efforts default per slot and, for the classifier, per the
	// provider that classifies, and Anthropic's web-tool caps default only
	// while their tool is on, so their values in force are resolved against
	// the file rather than read from a static default.
	cfg.withResolvedDefaults()
	applySectionDefaults(reflect.ValueOf(cfg).Elem())
	var buf bytes.Buffer
	if err := writeFull(&buf, reflect.ValueOf(cfg), 0); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func writeFull(buf *bytes.Buffer, v reflect.Value, depth int) error {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			if v.Kind() == reflect.Pointer && v.Type().Elem().Kind() == reflect.Struct {
				if isOptionalSection(v.Type().Elem()) {
					buf.WriteString("null") // absent is off; zeroed keys would be a broken section
					return nil
				}
				v = reflect.New(v.Type().Elem()) // a nil section, shown with its keys
				continue
			}
			buf.WriteString("null")
			return nil
		}
		v = v.Elem()
	}
	indent := strings.Repeat("  ", depth+1)
	closing := strings.Repeat("  ", depth)

	switch v.Kind() {
	case reflect.Struct:
		// A type that encodes itself (time.Time, a custom enum) keeps its form.
		if _, ok := v.Interface().(json.Marshaler); ok {
			return writeLeaf(buf, v)
		}
		t := v.Type()
		first := true
		buf.WriteString("{")
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" || !field.IsExported() {
				continue
			}
			if !first {
				buf.WriteString(",")
			}
			first = false
			fmt.Fprintf(buf, "\n%s%q: ", indent, name)
			if err := writeFull(buf, v.Field(i), depth+1); err != nil {
				return err
			}
		}
		if !first {
			buf.WriteString("\n" + closing)
		}
		buf.WriteString("}")
		return nil

	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return writeLeaf(buf, v)
		}
		keys := make([]string, 0, v.Len())
		for _, k := range v.MapKeys() {
			keys = append(keys, k.String())
		}
		sort.Strings(keys)
		buf.WriteString("{")
		for i, k := range keys {
			if i > 0 {
				buf.WriteString(",")
			}
			fmt.Fprintf(buf, "\n%s%q: ", indent, k)
			if err := writeFull(buf, v.MapIndex(reflect.ValueOf(k).Convert(v.Type().Key())), depth+1); err != nil {
				return err
			}
		}
		if len(keys) > 0 {
			buf.WriteString("\n" + closing)
		}
		buf.WriteString("}")
		return nil

	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			return writeLeaf(buf, v)
		}
		buf.WriteString("[")
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString("\n" + indent)
			if err := writeFull(buf, v.Index(i), depth+1); err != nil {
				return err
			}
		}
		if v.Len() > 0 {
			buf.WriteString("\n" + closing)
		}
		buf.WriteString("]")
		return nil
	}
	return writeLeaf(buf, v)
}

func writeLeaf(buf *bytes.Buffer, v reflect.Value) error {
	encoded, err := json.Marshal(v.Interface())
	if err != nil {
		return err
	}
	buf.Write(encoded)
	return nil
}

// applySectionDefaults replaces every section that has a value-receiver
// WithDefaults() with its defaulted copy, so the printout is the value in
// force. Absent sections are defaulted from their zero value; a section with
// no WithDefaults keeps what it has. Nested sections are handled the same way.
func applySectionDefaults(v reflect.Value) {
	if v.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}
		switch field.Kind() {
		case reflect.Pointer:
			if field.Type().Elem().Kind() != reflect.Struct {
				continue
			}
			section := reflect.New(field.Type().Elem()).Elem()
			if !field.IsNil() {
				section.Set(field.Elem())
			}
			defaulted, ok := withDefaults(section)
			if !ok {
				if field.IsNil() {
					continue
				}
				applySectionDefaults(field.Elem())
				continue
			}
			applySectionDefaults(defaulted)
			ptr := reflect.New(field.Type().Elem())
			ptr.Elem().Set(defaulted)
			field.Set(ptr)
		case reflect.Struct:
			if defaulted, ok := withDefaults(field); ok {
				field.Set(defaulted)
			}
			applySectionDefaults(field)
		}
	}
}

// withDefaults calls a value-receiver WithDefaults() that returns its own type.
func withDefaults(section reflect.Value) (reflect.Value, bool) {
	m := section.MethodByName("WithDefaults")
	if !m.IsValid() || m.Type().NumIn() != 0 || m.Type().NumOut() != 1 || m.Type().Out(0) != section.Type() {
		return reflect.Value{}, false
	}
	out := reflect.New(section.Type()).Elem()
	out.Set(m.Call(nil)[0])
	return out, true
}

// OptionalSection marks a section whose absence means the feature is off and
// whose presence must be complete. It has no defaults to print, so the full
// document writes it as null rather than as a zeroed section the loader would
// refuse, and the implicit-field pass does not call its keys defaulted.
type OptionalSection interface{ OffWhenAbsent() }

func isOptionalSection(t reflect.Type) bool {
	return reflect.PointerTo(t).Implements(reflect.TypeOf((*OptionalSection)(nil)).Elem())
}
