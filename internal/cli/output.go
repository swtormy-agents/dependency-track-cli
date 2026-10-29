package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"

	"github.com/itchyny/gojq"
)

func urlValues(m map[string]string) url.Values {
	if len(m) == 0 {
		return nil
	}
	q := make(url.Values, len(m))
	for k, v := range m {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q
}

func (a *app) emit(raw json.RawMessage) error {
	if a.jq == "" {
		return a.writeJSON(raw)
	}
	query, err := gojq.Parse(a.jq)
	if err != nil {
		return usageErr("--jq: %v", err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return usageErr("--jq: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var input any
	if err := dec.Decode(&input); err != nil {
		return runtimeErr(err)
	}
	iter := code.Run(normalize(input))
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if e, isErr := v.(error); isErr {
			return runtimeErr(fmt.Errorf("--jq: %v", e))
		}
		if s, isStr := v.(string); isStr {
			fmt.Fprintln(a.out, s)
			continue
		}
		b, err := marshal(v)
		if err != nil {
			return runtimeErr(err)
		}
		if err := a.writeJSON(b); err != nil {
			return err
		}
	}
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func (a *app) emitValue(v any) error {
	b, err := marshal(v)
	if err != nil {
		return runtimeErr(err)
	}
	return a.emit(b)
}

func (a *app) writeJSON(raw []byte) error {
	var buf bytes.Buffer
	var err error
	if a.pretty {
		err = json.Indent(&buf, raw, "", "  ")
	} else {
		err = json.Compact(&buf, raw)
	}
	if err != nil {
		return runtimeErr(err)
	}
	buf.WriteByte('\n')
	_, err = io.Copy(a.out, &buf)
	return err
}

func normalize(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil && i >= math.MinInt && i <= math.MaxInt {
			return int(i)
		}
		f, _ := t.Float64()
		return f
	case []any:
		for i := range t {
			t[i] = normalize(t[i])
		}
	case map[string]any:
		for k := range t {
			t[k] = normalize(t[k])
		}
	}
	return v
}
