package config

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/susji/jsonmetrics/internal/metric"
	"github.com/susji/tinyini"
	"k8s.io/client-go/util/jsonpath"
)

type Config struct {
	Metrics map[string]*metric.Metric
}

func New(r io.Reader) (*Config, error) {
	sections, errs := tinyini.Parse(r)
	if len(errs) > 0 {
		// If we get parse failures, we'll give up immediately.
		fmt.Println("config errors: ", errs)
		return nil, fmt.Errorf("configuration errors: %v", errs)
	}
	ret := &Config{}
	metrics := map[string]*metric.Metric{}
	// If parse was errorless, try consuming eagerly to figure out all errors.
	var reterr error
	sections.ForEach(func(section, k, v string) bool {
		switch {
		case section == "":
			log.Println("unrecognized key:", k)
		case strings.HasPrefix(section, "metric."):
			sn := strings.SplitN(section, ".", 2)
			if len(v) < 2 {
				log.Println("metric does not have a name")
				return true
			}
			name := sn[1]
			m, ok := metrics[name]
			if !ok {
				m = &metric.Metric{Name: name, Map: map[string]string{}}
			}
			switch k {
			case "metrictype":
				// FIXME check
				m.MetricType = v
			case "valuetype":
				// FIXME check
				m.ValueType = v
			case "help":
				m.Help = v
			case "valuepath":
				jp, err := parsepath(v, name)
				if err != nil {
					reterr = errors.Join(
						reterr,
						fmt.Errorf("bad valuepath: %w", err))
				}
				m.ValuePath = jp
			case "timestamppath":
				jp, err := parsepath(v, name)
				if err != nil {
					reterr = errors.Join(
						reterr,
						fmt.Errorf("bad timestamppath: %w", err))
				}
				m.TimestampPath = jp
			case "sourcepath":
				jp, err := parsepath(v, name)
				if err != nil {
					reterr = errors.Join(
						reterr,
						fmt.Errorf("bad sourcepath: %w", err))
				}
				m.SourcePath = jp
			case "source":
				m.Source = v
			case "timestampformat":
				m.TimestampFormat = v
			case "map":
				ms := strings.SplitN(v, ":", 2)
				m.Map[ms[0]] = ms[1]
			case "rendername":
				m.RenderName = v
			case "debounce":
				d, err := time.ParseDuration(v)
				if err != nil {
					reterr = errors.Join(
						reterr,
						fmt.Errorf("bad debounce %q: %w",
							v, err))
				}
				m.Debounce = &d
			default:
				reterr = errors.Join(
					reterr,
					fmt.Errorf(
						"unrecognized metric key for %q: %s",
						section, k))
			}
			metrics[name] = m
		default:
			reterr = errors.Join(reterr, fmt.Errorf("unrecognized section %q", section))
		}
		return true
	})
	// FIXME validate all metric values here
	if reterr != nil {
		return nil, reterr
	}
	ret.Metrics = metrics
	return ret, nil
}

func parsepath(val, name string) (*jsonpath.JSONPath, error) {
	jp := jsonpath.New(name)
	if err := jp.Parse(fixpath(val)); err != nil {
		return nil, err
	}
	return jp, nil
}

// k8s client uses the curly-braces wrapped format for JSONPath.
func fixpath(p string) string {
	if !strings.HasPrefix(p, "{") && !strings.HasPrefix(p, "}") {
		return "{" + p + "}"
	}
	return p
}
