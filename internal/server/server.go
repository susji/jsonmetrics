package server

import (
	"cmp"
	"fmt"
	"log"
	"net/http"

	"github.com/susji/jsonmetrics/internal/config"
	"github.com/susji/jsonmetrics/internal/metric"
	"github.com/susji/jsonmetrics/internal/misc"
	"github.com/susji/jsonmetrics/internal/state"
	"golang.org/x/exp/slices"
)

type MetricsOptions struct {
	ContentType string
	Endpoint    string
}

func GenerateRequestLogger(l *log.Logger, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// We assume that the logger will worry about timestamping.
		l.Printf(
			"%s %q %s %s",
			r.RemoteAddr,
			misc.StringOrDefault(r.Header.Get("x-forwarded-for"), "-"),
			r.Method,
			r.URL)
		h(w, r)
	}
}

func GenerateMetricsHandler(state *state.State, c *config.Config, mo MetricsOptions) http.HandlerFunc {
	ms := []*metric.Metric{}
	for _, m := range c.Metrics {
		ms = append(ms, m)
	}
	slices.SortFunc(ms, func(a, b *metric.Metric) int { return cmp.Compare(a.Name, b.Name) })
	return func(w http.ResponseWriter, r *http.Request) {
		// If performance ever becomes an issue, we could cache the metrics for
		// a configurable amount of time.
		w.Header().Add("content-type", mo.ContentType)
		for _, m := range ms {
			var name string
			if len(m.RenderName) > 0 {
				name = m.RenderName
			} else {
				name = m.Name
			}
			s, found := state.Get(name)
			if !found {
				continue
			}
			if mv, ok := m.Map[s.Value]; ok {
				s.Value = mv
			}
			// FIXME figure out how to print type and help
			fmt.Fprintf(w, "%s %s %d\n", name, s.Value, s.Timestamp.UnixMilli())
		}
	}
}
