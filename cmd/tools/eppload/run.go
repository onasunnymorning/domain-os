package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// options is one run of the tool.
type options struct {
	Addr      string
	TLS       *tls.Config
	Login     string
	Password  string
	Scenario  string // check, create or drop
	Sessions  int
	Duration  time.Duration
	Timeout   time.Duration
	Interval  time.Duration // between one session's commands
	Reconnect bool          // log in again when the server ends a session
	// check
	CheckNames int
	TLD        string
	// create, drop
	Create createSpec
	// drop
	Names []string
	At    time.Time // when the drop run starts sending creates
}

// sessionLog is what one session saw, merged into the report at the end.
type sessionLog struct {
	results map[string]map[int]int // command: code: count
	latency map[string][]time.Duration
	reasons map[string]int
	closes  int
	errors  int
}

func newSessionLog() *sessionLog {
	return &sessionLog{results: map[string]map[int]int{}, latency: map[string][]time.Duration{}, reasons: map[string]int{}}
}

func (l *sessionLog) add(cmd string, r result) {
	if l.results[cmd] == nil {
		l.results[cmd] = map[int]int{}
	}
	l.results[cmd][r.Code]++
	l.latency[cmd] = append(l.latency[cmd], r.Latency)
	if r.Reason != "" {
		l.reasons[r.Reason]++
	}
}

// drop is the shared state of a drop run: which names are still to be won.
type drop struct {
	mu    sync.Mutex
	order []string
	won   map[string]*nameResult
	tries map[string]int
}

// nameResult is one name of a drop run.
type nameResult struct {
	Name     string        `json:"name"`
	Won      bool          `json:"won"`
	WonAfter time.Duration `json:"won_after"`
	Session  int           `json:"session"`
	Attempts int           `json:"attempts"`
}

// next is the name session i should try, or "" when every one is won.
func (d *drop) next(i, turn int) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var left []string
	for _, n := range d.order {
		if d.won[n] == nil {
			left = append(left, n)
		}
	}
	if len(left) == 0 {
		return ""
	}
	n := left[(i+turn)%len(left)]
	d.tries[n]++
	return n
}

func (d *drop) record(name string, session int, after time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.won[name] == nil {
		d.won[name] = &nameResult{Name: name, Won: true, WonAfter: after, Session: session}
	}
}

func (d *drop) done() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.won) == len(d.order)
}

// run runs the scenario and returns its report.
func run(ctx context.Context, o options) (*report, error) {
	switch o.Scenario {
	case "check", "create":
	case "drop":
		if len(o.Names) == 0 {
			return nil, fmt.Errorf("a drop run needs names")
		}
	default:
		return nil, fmt.Errorf("unknown scenario %q: check, create or drop", o.Scenario)
	}
	var d *drop
	if o.Scenario == "drop" {
		d = &drop{order: slices.Clone(o.Names), won: map[string]*nameResult{}, tries: map[string]int{}}
		if wait := time.Until(o.At); wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, o.Duration)
	defer cancel()

	start := time.Now()
	var seq atomic.Int64
	logs := make([]*sessionLog, o.Sessions)
	var wg sync.WaitGroup
	for i := range o.Sessions {
		logs[i] = newSessionLog()
		wg.Add(1)
		go func() {
			defer wg.Done()
			session(ctx, o, i, logs[i], d, &seq, start, cancel)
		}()
	}
	wg.Wait()
	return buildReport(o, logs, d, time.Since(start)), nil
}

// session runs one EPP session until ctx is done, logging in again when
// the server ends it if o.Reconnect.
func session(ctx context.Context, o options, i int, log *sessionLog, d *drop, seq *atomic.Int64, start time.Time, stop func()) {
	trid := func() string { return "LOAD-" + strconv.FormatInt(seq.Add(1), 10) }
	turn := 0
	for ctx.Err() == nil {
		c, err := dialEPP(o.Addr, o.TLS, o.Timeout)
		if err != nil {
			log.errors++
			if !pause(ctx, time.Second) {
				return
			}
			continue
		}
		r, err := c.roundTrip(loginFrame(o.Login, o.Password, trid()))
		switch {
		case err != nil:
			log.errors++
		default:
			log.add("login", r)
		}
		if err != nil || r.Code != 1000 {
			c.close()
			if !o.Reconnect || !pause(ctx, 5*time.Second) {
				return
			}
			continue
		}
		for ctx.Err() == nil {
			var name, label string
			var frame []byte
			switch o.Scenario {
			case "check":
				names := make([]string, o.CheckNames)
				for k := range names {
					names[k] = fmt.Sprintf("load-%d-%d-%d.%s", i, turn, k, o.TLD)
				}
				label, frame = "domain:check", checkFrame(names, trid())
			case "create":
				name = fmt.Sprintf("load-%d-%d-%d.%s", start.Unix(), i, turn, o.TLD)
				label, frame = "domain:create", createFrame(name, o.Create, trid())
			case "drop":
				if name = d.next(i, turn); name == "" {
					stop()
					break
				}
				label, frame = "domain:create", createFrame(name, o.Create, trid())
			}
			if frame == nil {
				break
			}
			turn++
			r, err := c.roundTrip(frame)
			if err != nil {
				log.errors++
				break
			}
			log.add(label, r)
			if d != nil && r.Code == 1000 {
				d.record(name, i, time.Since(start))
				if d.done() {
					stop()
				}
			}
			if r.closing() {
				log.closes++
				break
			}
			if o.Interval > 0 && !pause(ctx, o.Interval) {
				break
			}
		}
		if ctx.Err() != nil {
			_, _ = c.roundTrip(logoutFrame(trid()))
		}
		c.close()
		if !o.Reconnect {
			return
		}
	}
}

// pause waits d, and reports false if ctx ended first.
func pause(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
