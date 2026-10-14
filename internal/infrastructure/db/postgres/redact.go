package postgres

import (
	"bytes"
	"encoding/json"
	"strings"
)

// RedactedMarker replaces the value of every authInfo field written to the
// event outbox. It is kept, rather than the field dropped, so consumers can see
// the field existed and was set.
const RedactedMarker = "[redacted]"

// redactSecrets marshals v to JSON with the value of every authInfo field, at
// any depth, replaced by RedactedMarker.
//
// It works on the encoded JSON rather than on Go types so that it covers every
// payload an event can carry — domain and contact entities, the commands that
// created them, slices of either, and whatever a future producer passes in —
// without each producer having to remember to redact. Keys are matched
// case-insensitively, ignoring '_' and '-', so "AuthInfo", "authInfo" and
// "auth_info" all match. Empty and null values are left as they are.
func redactSecrets(v interface{}) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // keep large integers exact through the round trip
	var tree interface{}
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	if !redactTree(tree) {
		return raw, nil
	}
	return json.Marshal(tree)
}

// redactTree replaces authInfo values in place and reports whether it changed
// anything.
func redactTree(node interface{}) bool {
	changed := false
	switch n := node.(type) {
	case map[string]interface{}:
		for k, child := range n {
			if isSecretKey(k) {
				if !isEmptyJSON(child) {
					n[k] = RedactedMarker
					changed = true
				}
				continue
			}
			if redactTree(child) {
				changed = true
			}
		}
	case []interface{}:
		for _, child := range n {
			if redactTree(child) {
				changed = true
			}
		}
	}
	return changed
}

func isSecretKey(k string) bool {
	k = strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(k))
	return k == "authinfo"
}

func isEmptyJSON(v interface{}) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}
