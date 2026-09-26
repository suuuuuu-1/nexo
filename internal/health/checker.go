// Package health runs bounded readiness probes for the services required by Nexo.
package health

import (
	"context"
	"errors"
)

// Probe checks one dependency. Probes should honor the supplied context.
type Probe func(context.Context) error

// Checker executes dependency probes concurrently so one slow dependency does
// not multiply the total readiness-check latency.
type Checker struct {
	probes map[string]Probe
}

// Report is safe to expose from the readiness endpoint: it reports status, not
// internal connection strings or low-level error details.
type Report struct {
	Status       string            `json:"status"`
	Dependencies map[string]string `json:"dependencies"`
}

// NewChecker copies the probe map so callers cannot change it after startup.
func NewChecker(probes map[string]Probe) *Checker {
	copyOfProbes := make(map[string]Probe, len(probes))
	for name, probe := range probes {
		copyOfProbes[name] = probe
	}
	return &Checker{probes: copyOfProbes}
}

// Check runs every dependency probe and marks failed or timed-out dependencies
// as unavailable. The caller controls the overall deadline through ctx.
func (c *Checker) Check(ctx context.Context) Report {
	if c == nil || len(c.probes) == 0 {
		return Report{Status: "not_ready", Dependencies: map[string]string{}}
	}

	type probeResult struct {
		name string
		err  error
	}
	results := make(chan probeResult, len(c.probes))
	for name, probe := range c.probes {
		go func(name string, probe Probe) {
			if probe == nil {
				results <- probeResult{name: name, err: errors.New("probe is nil")}
				return
			}
			results <- probeResult{name: name, err: probe(ctx)}
		}(name, probe)
	}

	dependencies := make(map[string]string, len(c.probes))
	for completed := 0; completed < len(c.probes); {
		select {
		case result := <-results:
			completed++
			if result.err == nil {
				dependencies[result.name] = "ok"
			} else {
				dependencies[result.name] = "unavailable"
			}
		case <-ctx.Done():
			for name := range c.probes {
				if _, finished := dependencies[name]; !finished {
					dependencies[name] = "unavailable"
				}
			}
			return makeReport(dependencies)
		}
	}
	return makeReport(dependencies)
}

func makeReport(dependencies map[string]string) Report {
	for _, status := range dependencies {
		if status != "ok" {
			return Report{Status: "not_ready", Dependencies: dependencies}
		}
	}
	return Report{Status: "ready", Dependencies: dependencies}
}
