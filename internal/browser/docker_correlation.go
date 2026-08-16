package browser

import (
	"context"
	"fmt"
	"sort"
	"time"

	"codenerd/internal/browser/security"
)

// Container log correlation is bounded on every axis. Container logs are
// attacker-influenced and effectively unbounded, and they are fetched
// during a diagnosis the agent is actively waiting on. Capping how many
// containers are consulted, how much log is scanned per container, and how
// many correlations are returned keeps a single diagnosis from reading
// arbitrary disk, stalling the agent on a large fetch, or flooding the
// kernel with derived facts.
const (
	maxCorrelationContainers = 8
	maxContainerLogLines     = 500
	maxContainerCorrelations = 50
)

// ContainerLogLine is one line from a container's log stream.
type ContainerLogLine struct {
	Container string
	Timestamp time.Time
	Message   string
}

// RuntimeErrorEvent is a browser-side event that a container log line may
// explain. It is intentionally neutral and does not import mangle or fact
// types; the wiring layer adapts kernel facts into this shape.
type RuntimeErrorEvent struct {
	Kind      string
	Detail    string
	Timestamp time.Time
}

// ContainerLogFetcher retrieves log lines for a single container within a
// bounded time window. Implementations must respect ctx cancellation and
// return an error without panicking when the container is missing or the
// daemon is unavailable. Returning an error is non-fatal to correlation:
// the caller records a Note and continues with the remaining containers.
type ContainerLogFetcher func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error)

// ContainerCorrelation pairs a container log line with a browser runtime
// error that occurred close in time.
type ContainerCorrelation struct {
	Container    string
	LogMessage   string
	LogTimestamp time.Time
	EventKind    string
	EventDetail  string
	DeltaMs      int64 // signed: negative means container logged BEFORE the browser event
}

// ContainerCorrelationResult holds the bounded output of a correlation pass.
type ContainerCorrelationResult struct {
	Correlations []ContainerCorrelation
	Notes        []string
	Truncated    bool
}

// ContainerCorrelationRequest carries the inputs of one correlation pass.
// It keeps the correlation call within the project's five-parameter
// standard and lets the inputs grow without re-breaking the guard.
type ContainerCorrelationRequest struct {
	Fetcher    ContainerLogFetcher
	Containers []string
	Events     []RuntimeErrorEvent
	Window     time.Duration
	Redactor   *security.Redactor
}

// CorrelateContainerLogs correlates container logs with browser runtime
// errors that occurred within window of each other. The fetcher is
// injectable so the core has no hard dependency on a Docker daemon and
// tests can run without one.
func CorrelateContainerLogs(ctx context.Context, req ContainerCorrelationRequest) ContainerCorrelationResult {
	if len(req.Containers) == 0 || len(req.Events) == 0 || req.Fetcher == nil {
		if req.Fetcher == nil && len(req.Containers) > 0 {
			return ContainerCorrelationResult{Notes: []string{"correlation requested but no container log source is configured"}}
		}
		return ContainerCorrelationResult{}
	}
	result := ContainerCorrelationResult{}
	clamped, wasTruncated, note := clampContainers(req.Containers)
	if wasTruncated {
		result.Truncated = true
		result.Notes = append(result.Notes, note)
	}
	window, since, until := correlationWindow(req.Events, req.Window)
	redactor := req.Redactor
	if redactor == nil {
		redactor = security.NewRedactor(nil)
	}
	var correlations []ContainerCorrelation
	for _, container := range clamped {
		if ctx.Err() != nil {
			result.Notes = append(result.Notes, fmt.Sprintf("context cancelled before fetching container %q: %v", container, ctx.Err()))
			break
		}
		lines, err := req.Fetcher(ctx, container, since, until)
		if err != nil {
			result.Notes = append(result.Notes, fmt.Sprintf("container %q: %v", container, err))
			continue
		}
		if len(lines) > maxContainerLogLines {
			orig := len(lines)
			lines = lines[orig-maxContainerLogLines:]
			result.Truncated = true
			truncNote := fmt.Sprintf("truncated log lines: kept newest %d of %d (limit %d)", maxContainerLogLines, orig, maxContainerLogLines)
			if len(lines) > 0 && lines[0].Container != "" {
				truncNote = fmt.Sprintf("truncated log lines for container %q: kept newest %d of %d (limit %d)", lines[0].Container, maxContainerLogLines, orig, maxContainerLogLines)
			}
			result.Notes = append(result.Notes, truncNote)
		}
		correlations = append(correlations, correlationsForContainer(container, lines, req.Events, window, redactor)...)
	}
	sortCorrelations(correlations)
	if len(correlations) > maxContainerCorrelations {
		correlations = correlations[:maxContainerCorrelations]
		result.Truncated = true
		result.Notes = append(result.Notes, fmt.Sprintf("truncated correlations: limit %d reached", maxContainerCorrelations))
	}
	result.Correlations = correlations
	if result.Correlations == nil {
		result.Correlations = []ContainerCorrelation{}
	}
	return result
}

func clampContainers(containers []string) (clamped []string, truncated bool, note string) {
	if len(containers) <= maxCorrelationContainers {
		return containers, false, ""
	}
	skipped := len(containers) - maxCorrelationContainers
	clamped = containers[:maxCorrelationContainers]
	note = fmt.Sprintf("truncated containers: skipped %d containers (limit %d)", skipped, maxCorrelationContainers)
	return clamped, true, note
}

func correlationWindow(events []RuntimeErrorEvent, window time.Duration) (time.Duration, time.Time, time.Time) {
	if window <= 0 {
		window = 5 * time.Second
	}
	earliest := events[0].Timestamp
	latest := events[0].Timestamp
	for _, ev := range events[1:] {
		if ev.Timestamp.Before(earliest) {
			earliest = ev.Timestamp
		}
		if ev.Timestamp.After(latest) {
			latest = ev.Timestamp
		}
	}
	since := earliest.Add(-window)
	until := latest.Add(window)
	return window, since, until
}

func correlationsForContainer(container string, lines []ContainerLogLine, events []RuntimeErrorEvent, window time.Duration, redactor *security.Redactor) []ContainerCorrelation {
	var out []ContainerCorrelation
	for _, line := range lines {
		for _, ev := range events {
			delta := line.Timestamp.Sub(ev.Timestamp)
			if delta < -window || delta > window {
				continue
			}
			corrContainer := container
			if line.Container != "" {
				corrContainer = line.Container
			}
			out = append(out, ContainerCorrelation{
				Container:    corrContainer,
				LogMessage:   redactor.SanitizeString(line.Message),
				LogTimestamp: line.Timestamp,
				EventKind:    ev.Kind,
				EventDetail:  redactor.SanitizeString(ev.Detail),
				DeltaMs:      delta.Milliseconds(),
			})
		}
	}
	return out
}

func sortCorrelations(correlations []ContainerCorrelation) {
	sort.Slice(correlations, func(i, j int) bool {
		ai := correlations[i].DeltaMs
		if ai < 0 {
			ai = -ai
		}
		aj := correlations[j].DeltaMs
		if aj < 0 {
			aj = -aj
		}
		if ai != aj {
			return ai < aj
		}
		if correlations[i].Container != correlations[j].Container {
			return correlations[i].Container < correlations[j].Container
		}
		return correlations[i].LogMessage < correlations[j].LogMessage
	})
}
