package cli

import (
	"encoding/json"
)

// untrustedFieldNames marks third-party text in Dependency-Track payloads.
var untrustedFieldNames = map[string]bool{
	"description": true,
	"details":     true,
	"comment":     true,
	"justification": true,
	"analysis":    true,
	"recommendation": true,
	"title":       true,
}

func (a *app) emitUntrusted(raw json.RawMessage) error {
	if !a.untrusted {
		return a.emit(raw)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return a.emit(raw)
	}
	markUntrusted(v)
	b, err := marshal(v)
	if err != nil {
		return runtimeErr(err)
	}
	return a.emit(b)
}

func markUntrusted(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if untrustedFieldNames[k] {
				if s, ok := val.(string); ok && s != "" {
					t[k] = map[string]any{"untrusted": true, "text": s}
				}
			} else {
				markUntrusted(val)
			}
		}
	case []any:
		for i := range t {
			markUntrusted(t[i])
		}
	}
}
