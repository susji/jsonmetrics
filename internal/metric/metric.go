package metric

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"k8s.io/client-go/util/jsonpath"
)

type Metric struct {
	Source          string
	MetricType      string
	ValueType       string
	Name            string
	Help            string
	TimestampFormat string
	RenderName      string
	Debounce        *time.Duration
	Map             map[string]string
	TimestampPath   *jsonpath.JSONPath
	ValuePath       *jsonpath.JSONPath
	SourcePath      *jsonpath.JSONPath
}

func parse(src string, jp *jsonpath.JSONPath) (string, error) {
	if jp == nil {
		return "", errors.New("jsonpath missing")
	}
	var v interface{}
	if err := json.Unmarshal([]byte(src), &v); err != nil {
		return "", err
	}
	buf := &bytes.Buffer{}
	if err := jp.Execute(buf, v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (m *Metric) ParseValue(src string) (string, error) {
	return parse(src, m.ValuePath)
}

func (m *Metric) ParseSource(src string) (string, error) {
	return parse(src, m.SourcePath)
}

func (m *Metric) ParseTimestamp(src string) (time.Time, error) {
	raw, err := parse(src, m.TimestampPath)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(m.TimestampFormat, raw)
}
