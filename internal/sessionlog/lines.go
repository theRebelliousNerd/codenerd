package sessionlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Lines splits text into the array stored in files. Join restores it exactly,
// including a trailing newline (an empty last element) and "\r" line ends.
func Lines(s string) []string { return strings.Split(s, "\n") }

// Join is the inverse of Lines.
func Join(lines []string) string { return strings.Join(lines, "\n") }

// Object is a decoded JSON object that remembers its key order. Wire bodies
// are stored through it so the file shows the bytes' order, which is what
// decides a provider cache prefix.
type Object struct {
	Keys   []string
	Values map[string]any
}

// Get returns a member.
func (o *Object) Get(key string) (any, bool) {
	if o == nil {
		return nil, false
	}
	v, ok := o.Values[key]
	return v, ok
}

// MarshalJSON writes members in their original order.
func (o *Object) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.Keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := marshalPlain(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := marshalPlain(o.Values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// DecodeOrdered decodes one JSON document into strings, json.Number, bool,
// nil, []any and *Object. Numbers keep their literal text.
func DecodeOrdered(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data after JSON value")
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return tok, nil
	}
	switch delim {
	case '{':
		obj := &Object{Values: map[string]any{}}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, _ := kt.(string)
			val, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			if _, dup := obj.Values[key]; !dup {
				obj.Keys = append(obj.Keys, key)
			}
			obj.Values[key] = val
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return obj, nil
	case '[':
		arr := []any{}
		for dec.More() {
			val, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, val)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return arr, nil
	}
	return nil, fmt.Errorf("unexpected delimiter %v", delim)
}

// SplitLines walks a decoded value and replaces every string that contains a
// newline with the array of its lines. It returns the converted value and the
// JSON pointer (RFC 6901) of every string it replaced, so RestoreTree can tell
// a split string from an array that was an array on the wire. v is not
// modified.
func SplitLines(v any) (any, []string) {
	var paths []string
	out := splitWalk(v, "", &paths)
	return out, paths
}

func splitWalk(v any, path string, paths *[]string) any {
	switch t := v.(type) {
	case string:
		if !strings.Contains(t, "\n") {
			return t
		}
		*paths = append(*paths, path)
		lines := Lines(t)
		out := make([]any, len(lines))
		for i, l := range lines {
			out[i] = l
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = splitWalk(e, path+"/"+strconv.Itoa(i), paths)
		}
		return out
	case *Object:
		out := &Object{Keys: append([]string(nil), t.Keys...), Values: make(map[string]any, len(t.Values))}
		for _, k := range t.Keys {
			out.Values[k] = splitWalk(t.Values[k], path+"/"+escapePointer(k), paths)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = splitWalk(e, path+"/"+escapePointer(k), paths)
		}
		return out
	default:
		return v
	}
}

// RestoreTree is the inverse of SplitLines: it joins the arrays at paths back
// into the strings they were. It works on values decoded by DecodeOrdered and
// does not modify v.
func RestoreTree(v any, paths []string) (any, error) {
	want := make(map[string]bool, len(paths))
	for _, p := range paths {
		want[p] = true
	}
	var err error
	out := restoreWalk(v, "", want, &err)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func restoreWalk(v any, path string, want map[string]bool, errOut *error) any {
	if want[path] {
		arr, ok := v.([]any)
		if !ok {
			if *errOut == nil {
				*errOut = fmt.Errorf("lines path %q is not an array", path)
			}
			return v
		}
		lines := make([]string, len(arr))
		for i, e := range arr {
			s, ok := e.(string)
			if !ok {
				if *errOut == nil {
					*errOut = fmt.Errorf("lines path %q element %d is not a string", path, i)
				}
				return v
			}
			lines[i] = s
		}
		return Join(lines)
	}
	switch t := v.(type) {
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = restoreWalk(e, path+"/"+strconv.Itoa(i), want, errOut)
		}
		return out
	case *Object:
		out := &Object{Keys: append([]string(nil), t.Keys...), Values: make(map[string]any, len(t.Values))}
		for _, k := range t.Keys {
			out.Values[k] = restoreWalk(t.Values[k], path+"/"+escapePointer(k), want, errOut)
		}
		return out
	default:
		return v
	}
}

func escapePointer(k string) string {
	k = strings.ReplaceAll(k, "~", "~0")
	return strings.ReplaceAll(k, "/", "~1")
}

// ToTree turns any JSON-encodable value into the ordered tree SplitLines
// walks (struct fields keep their declaration order).
func ToTree(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return DecodeOrdered(data)
}

// marshalPlain is json.Marshal without HTML escaping: a prompt full of <tags>
// and & stays readable in the file.
func marshalPlain(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// MarshalFile renders v the way every file in a session folder is written:
// 2-space indent, no HTML escaping, a trailing newline.
func MarshalFile(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
