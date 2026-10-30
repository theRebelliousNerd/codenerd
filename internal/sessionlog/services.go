package sessionlog

import (
	"sort"
	"time"
)

// ServiceStatus is process-resident health, not a task's execution progress.
// LastActivity is an observation timestamp, never a running duration.
type ServiceStatus struct {
	Name         string
	Health       string
	LastActivity time.Time
}

type serviceState struct {
	status ServiceStatus
	active map[string]struct{}
}

// Services returns a detached, name-sorted snapshot across chat sessions.
// The registry keeps one row per service role, not one per activation.
func (s *Store) Services() []ServiceStatus {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ServiceStatus, 0, len(s.services))
	for _, service := range s.services {
		out = append(out, service.status)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) observeService(scope *Scope, ev LiveEvent) {
	if s == nil || scope.Role == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.services == nil {
		s.services = make(map[string]*serviceState)
	}
	service := s.services[scope.Role]
	if service == nil {
		service = &serviceState{status: ServiceStatus{Name: scope.Role}, active: make(map[string]struct{})}
		s.services[scope.Role] = service
	}
	service.status.LastActivity = time.Now()
	service.status.Health = "ready"
	if ev.Kind == EventFinal {
		delete(service.active, scope.Folder)
		if len(service.active) == 0 {
			service.status.Health = "stopped"
		}
	} else {
		service.active[scope.Folder] = struct{}{}
	}
	if ev.Failed {
		service.status.Health = "failed"
	}
}
