package main

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"
)

// report is a run's results.
type report struct {
	Scenario string                    `json:"scenario"`
	Sessions int                       `json:"sessions"`
	Elapsed  time.Duration             `json:"elapsed"`
	Commands map[string]*commandReport `json:"commands"`
	// Reasons counts the <extValue> reasons of refusals: rate limits,
	// failed creates, the create queue.
	Reasons map[string]int `json:"reasons,omitempty"`
	// Closes is how many times the server ended a session; Errors how
	// many connections or responses failed.
	Closes int           `json:"closes"`
	Errors int           `json:"errors"`
	Names  []*nameResult `json:"names,omitempty"`
}

// commandReport is one command's results.
type commandReport struct {
	Count     int           `json:"count"`
	PerSecond float64       `json:"per_second"`
	Codes     map[int]int   `json:"codes"`
	P50       time.Duration `json:"p50"`
	P95       time.Duration `json:"p95"`
	P99       time.Duration `json:"p99"`
	Max       time.Duration `json:"max"`
	latencies []time.Duration
}

func quantile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[min(len(sorted)-1, int(q*float64(len(sorted))))]
}

func buildReport(o options, logs []*sessionLog, d *drop, elapsed time.Duration) *report {
	rep := &report{Scenario: o.Scenario, Sessions: o.Sessions, Elapsed: elapsed, Commands: map[string]*commandReport{}, Reasons: map[string]int{}}
	for _, l := range logs {
		rep.Closes += l.closes
		rep.Errors += l.errors
		for reason, n := range l.reasons {
			// Reasons carry numbers (retry-after-ms=…): count by kind.
			kind, _, _ := strings.Cut(reason, ";")
			rep.Reasons[kind] += n
		}
		for cmd, codes := range l.results {
			cr := rep.Commands[cmd]
			if cr == nil {
				cr = &commandReport{Codes: map[int]int{}}
				rep.Commands[cmd] = cr
			}
			for code, n := range codes {
				cr.Codes[code] += n
				cr.Count += n
			}
			cr.latencies = append(cr.latencies, l.latency[cmd]...)
		}
	}
	for _, cr := range rep.Commands {
		slices.Sort(cr.latencies)
		cr.P50, cr.P95, cr.P99 = quantile(cr.latencies, 0.5), quantile(cr.latencies, 0.95), quantile(cr.latencies, 0.99)
		if n := len(cr.latencies); n > 0 {
			cr.Max = cr.latencies[n-1]
		}
		if elapsed > 0 {
			cr.PerSecond = float64(cr.Count) / elapsed.Seconds()
		}
	}
	if d != nil {
		for _, n := range d.order {
			r := d.won[n]
			if r == nil {
				r = &nameResult{Name: n}
			}
			r.Attempts = d.tries[n]
			rep.Names = append(rep.Names, r)
		}
	}
	return rep
}

// print writes the report for a person to read.
func (r *report) print(w io.Writer) {
	fmt.Fprintf(w, "%s: %d sessions, %s\n", r.Scenario, r.Sessions, r.Elapsed.Round(time.Millisecond))
	cmds := make([]string, 0, len(r.Commands))
	for c := range r.Commands {
		cmds = append(cmds, c)
	}
	sort.Strings(cmds)
	for _, c := range cmds {
		cr := r.Commands[c]
		codes := make([]int, 0, len(cr.Codes))
		for code := range cr.Codes {
			codes = append(codes, code)
		}
		sort.Ints(codes)
		var parts []string
		for _, code := range codes {
			parts = append(parts, fmt.Sprintf("%d×%d", code, cr.Codes[code]))
		}
		fmt.Fprintf(w, "  %-14s %7d (%.1f/s)  p50 %s  p95 %s  p99 %s  max %s  [%s]\n", c, cr.Count, cr.PerSecond,
			cr.P50.Round(time.Microsecond), cr.P95.Round(time.Microsecond), cr.P99.Round(time.Microsecond), cr.Max.Round(time.Microsecond), strings.Join(parts, " "))
	}
	if len(r.Reasons) > 0 {
		kinds := make([]string, 0, len(r.Reasons))
		for k := range r.Reasons {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		fmt.Fprint(w, "  refusal reasons:")
		for _, k := range kinds {
			fmt.Fprintf(w, " %s×%d", k, r.Reasons[k])
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "  sessions ended by the server: %d, connection errors: %d\n", r.Closes, r.Errors)
	if len(r.Names) > 0 {
		won := 0
		for _, n := range r.Names {
			if n.Won {
				won++
			}
		}
		fmt.Fprintf(w, "  names: %d of %d won\n", won, len(r.Names))
		for _, n := range r.Names {
			if n.Won {
				fmt.Fprintf(w, "    %-30s won after %s by session %d, %d attempts\n", n.Name, n.WonAfter.Round(time.Millisecond), n.Session, n.Attempts)
			} else {
				fmt.Fprintf(w, "    %-30s not won, %d attempts\n", n.Name, n.Attempts)
			}
		}
	}
}
