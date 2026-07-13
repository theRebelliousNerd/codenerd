package tactile

import (
	"errors"
	"testing"
	"time"
)

func TestDockerDetectionCachesUnavailableProbe(t *testing.T) {
	dockerDetectionCache.Lock()
	previousCheckedAt := dockerDetectionCache.checkedAt
	previousPath := dockerDetectionCache.path
	previousAvailable := dockerDetectionCache.available
	dockerDetectionCache.checkedAt = time.Time{}
	dockerDetectionCache.path = ""
	dockerDetectionCache.available = false
	dockerDetectionCache.Unlock()

	previousLookPath := dockerLookPath
	probes := 0
	dockerLookPath = func(string) (string, error) {
		probes++
		return "", errors.New("docker unavailable")
	}
	t.Cleanup(func() {
		dockerLookPath = previousLookPath
		dockerDetectionCache.Lock()
		dockerDetectionCache.checkedAt = previousCheckedAt
		dockerDetectionCache.path = previousPath
		dockerDetectionCache.available = previousAvailable
		dockerDetectionCache.Unlock()
	})

	first := NewDockerExecutorWithConfig(DefaultExecutorConfig())
	second := NewDockerExecutorWithConfig(DefaultExecutorConfig())
	if first.IsAvailable() || second.IsAvailable() {
		t.Fatal("unavailable Docker probe was reported as available")
	}
	if probes != 1 {
		t.Fatalf("Docker availability probed %d times, want one cached probe", probes)
	}
}

func TestCompositeExecutorPassesConfigToDockerProbe(t *testing.T) {
	dockerDetectionCache.Lock()
	previousCheckedAt := dockerDetectionCache.checkedAt
	previousPath := dockerDetectionCache.path
	previousAvailable := dockerDetectionCache.available
	dockerDetectionCache.checkedAt = time.Now()
	dockerDetectionCache.path = "docker-test"
	dockerDetectionCache.available = true
	dockerDetectionCache.Unlock()
	t.Cleanup(func() {
		dockerDetectionCache.Lock()
		dockerDetectionCache.checkedAt = previousCheckedAt
		dockerDetectionCache.path = previousPath
		dockerDetectionCache.available = previousAvailable
		dockerDetectionCache.Unlock()
	})

	config := DefaultExecutorConfig()
	config.DefaultTimeout = 17 * time.Second
	composite := NewCompositeExecutorWithConfig(config)
	docker, ok := composite.executors[SandboxDocker].(*DockerExecutor)
	if !ok {
		t.Fatal("cached available Docker executor was not registered")
	}
	if got := docker.config.DefaultTimeout; got != config.DefaultTimeout {
		t.Fatalf("Docker default timeout = %s, want %s", got, config.DefaultTimeout)
	}
}

func TestCompositeExecutorExplicitMissingSandboxFailsClosed(t *testing.T) {
	config := DefaultExecutorConfig()
	composite := NewCompositeExecutorWithConfig(config)
	delete(composite.executors, SandboxDocker)

	command := Command{
		Binary:  "go",
		Sandbox: &SandboxConfig{Mode: SandboxDocker},
	}
	if executor := composite.selectExecutor(command); executor != nil {
		t.Fatalf("explicit unavailable Docker request selected %T; want nil", executor)
	}
	if _, err := composite.Execute(t.Context(), command); err == nil {
		t.Fatal("explicit unavailable Docker request executed; want fail-closed error")
	}
}

func TestExecutorFactoryCreateDockerPassesConfig(t *testing.T) {
	dockerDetectionCache.Lock()
	previousCheckedAt := dockerDetectionCache.checkedAt
	previousPath := dockerDetectionCache.path
	previousAvailable := dockerDetectionCache.available
	dockerDetectionCache.checkedAt = time.Now()
	dockerDetectionCache.path = "docker-test"
	dockerDetectionCache.available = true
	dockerDetectionCache.Unlock()
	t.Cleanup(func() {
		dockerDetectionCache.Lock()
		dockerDetectionCache.checkedAt = previousCheckedAt
		dockerDetectionCache.path = previousPath
		dockerDetectionCache.available = previousAvailable
		dockerDetectionCache.Unlock()
	})

	config := DefaultExecutorConfig()
	config.DefaultTimeout = 23 * time.Second
	factory := NewExecutorFactory(config)
	docker, err := factory.CreateDocker()
	if err != nil {
		t.Fatalf("CreateDocker error = %v", err)
	}
	if docker.config.DefaultTimeout != config.DefaultTimeout {
		t.Fatalf("Docker default timeout = %s, want %s", docker.config.DefaultTimeout, config.DefaultTimeout)
	}
}
