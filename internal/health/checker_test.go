package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCheckerReportsReadyWhenAllProbesPass(t *testing.T) {
	checker := NewChecker(map[string]Probe{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return nil },
	})

	report := checker.Check(context.Background())
	if report.Status != "ready" {
		t.Fatalf("status = %q, want ready", report.Status)
	}
	if report.Dependencies["postgres"] != "ok" || report.Dependencies["redis"] != "ok" {
		t.Fatalf("unexpected dependencies: %#v", report.Dependencies)
	}
}

func TestCheckerReportsUnavailableDependency(t *testing.T) {
	checker := NewChecker(map[string]Probe{
		"postgres": func(context.Context) error { return errors.New("private connection detail") },
	})

	report := checker.Check(context.Background())
	if report.Status != "not_ready" || report.Dependencies["postgres"] != "unavailable" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestCheckerRespectsDeadline(t *testing.T) {
	checker := NewChecker(map[string]Probe{
		"slow": func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	report := checker.Check(ctx)
	if report.Status != "not_ready" || report.Dependencies["slow"] != "unavailable" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestCheckerWithNoProbesIsNotReady(t *testing.T) {
	report := NewChecker(nil).Check(context.Background())
	if report.Status != "not_ready" {
		t.Fatalf("status = %q, want not_ready", report.Status)
	}
}
