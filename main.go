package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/susji/jsonmetrics/internal/config"
	"github.com/susji/jsonmetrics/internal/server"
	"github.com/susji/jsonmetrics/internal/state"
)

func main() {
	rc := &runcontext{
		l: log.Default(),
	}
	cp := flag.String("config", "", "Path to the configuration file")
	flag.StringVar(&rc.b, "listen", "localhost:19100", "Listening address for HTTP server")
	flag.BoolVar(&rc.lr, "logrequests", false, "Log HTTP requests")
	flag.BoolVar(&rc.li, "loginput", false, "Log input")
	flag.Parse()
	if len(*cp) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	cf, err := os.Open(*cp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open configuration:", err)
		os.Exit(1)
	}
	rc.c, err = config.New(cf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration errors:", err)
		os.Exit(1)
	}
	rc.handle()
}

type runcontext struct {
	b  string
	lr bool
	li bool
	c  *config.Config
	l  *log.Logger
}

func (rc *runcontext) handle() {
	st := state.New()
	wg := &sync.WaitGroup{}
	wg.Add(2)
	s := &http.Server{Addr: rc.b}
	h := server.GenerateMetricsHandler(st, rc.c, server.MetricsOptions{
		ContentType: "text/plain; version=0.0.4",
		Endpoint:    "/metrics",
	})
	if rc.lr {
		h = server.GenerateRequestLogger(rc.l, h)
	}
	http.HandleFunc("/metrics", h)
	go func() {
		defer wg.Done()
		ret := 0
		err := s.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			rc.l.Println("HTTP server closed - reader probably shut down")
		} else if err != nil {
			rc.l.Println("HTTP server errored:", err)
			ret = 1
		}
		os.Exit(ret)
	}()
	go func() {
		defer wg.Done()
		defer s.Close()
		s := bufio.NewScanner(os.Stdin)
		rc.l.Println("reading standard input")
		for s.Scan() {
			rc.l.Println(s.Text())
			var w interface{}
			err := json.Unmarshal(s.Bytes(), &w)
			if err != nil {
				rc.l.Println("cannot unmarshal JSON:", err)
				continue
			}
			for _, metric := range rc.c.Metrics {
				src, err := metric.ParseSource(s.Text())
				if err != nil {
					rc.l.Println("bad source:", err)
					continue
				}
				if src != metric.Source {
					continue
				}
				rc.l.Println("source:", src)
				val, err := metric.ParseValue(s.Text())
				if err != nil {
					rc.l.Println("bad value:", err)
					continue
				}
				rc.l.Println("value:", val)
				var ts *time.Time
				if metric.TimestampPath != nil {
					tsp, err := metric.ParseTimestamp(s.Text())
					if err != nil {
						rc.l.Println("bad timestamp:", err)
						continue
					}
					ts = &tsp
					rc.l.Println("timestamp:", *ts)
				}
				var name string
				if len(metric.RenderName) > 0 {
					name = metric.RenderName
				} else {
					name = metric.Name
				}
				st.Update(name, val, metric.Debounce, ts)
			}
		}
		if err := s.Err(); err != nil {
			rc.l.Println("reading input failed:", err)
		}
	}()
	wg.Wait()
}
