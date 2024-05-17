package state

import (
	"log"
	"sync"
	"time"
)

type State struct {
	v map[string]Sample
	d map[string]debouncedsample
	m *sync.RWMutex
}

type Sample struct {
	Value     string
	Timestamp time.Time
}

type debouncedsample struct {
	value   string
	until   time.Time
	sampled time.Time
}

func (s *State) Update(name, value string, db *time.Duration, tnew *time.Time) error {
	s.m.Lock()
	defer s.m.Unlock()
	if tnew == nil {
		now := time.Now()
		tnew = &now
	}
	tprev := s.v[name].Timestamp
	oldval, gotoldval := s.v[name]
	// If the metric has a debounce duration, check it if the value has
	// changed.
	if db != nil && (!gotoldval || (gotoldval && value != oldval.Value)) {
		ds, debouncing := s.d[name]
		if !debouncing || (debouncing && tnew.After(ds.until)) {
			s.d[name] = debouncedsample{
				value:   value,
				until:   tnew.Add(*db),
				sampled: *tnew}
		}
	}
	if tnew.After(tprev) {
		s.v[name] = Sample{Value: value, Timestamp: *tnew}
		log.Println("updated", name, "to", value)
	}
	return nil
}

func (s *State) Get(name string) (Sample, bool) {
	s.m.RLock()
	defer s.m.RUnlock()
	if ds, debouncing := s.d[name]; debouncing {
		if time.Now().Before(ds.until) {
			log.Printf("%q debouncing at %q until %v", name, ds.value, ds.until)
			return Sample{Value: ds.value, Timestamp: ds.sampled}, true
		}
	}
	sa, ok := s.v[name]
	return sa, ok
}

func New() *State {
	return &State{
		v: map[string]Sample{},
		d: map[string]debouncedsample{},
		m: &sync.RWMutex{},
	}
}
