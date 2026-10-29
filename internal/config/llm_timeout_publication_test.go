package config

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLLMTimeoutPublication_ConcurrentCompleteSnapshots(t *testing.T) {
	llmTimeoutPublicationFixture(t)
	profiles := []LLMTimeouts{
		DefaultLLMTimeouts(),
		llmTimeoutPublicationProfile(11*time.Second, 0),
		llmTimeoutPublicationProfile(101*time.Second, 7),
	}
	accepted := make(map[LLMTimeouts]bool, len(profiles))
	for _, profile := range profiles {
		accepted[profile] = true
		SetLLMTimeouts(profile)
		require.Equal(t, profile, GetLLMTimeouts())
	}
	SetLLMTimeouts(profiles[0])

	const readers = 8
	const iterations = 2000
	start := make(chan struct{})
	invalid := make(chan LLMTimeouts, readers)
	var ready, workers sync.WaitGroup
	ready.Add(len(profiles) + readers)
	workers.Add(len(profiles) + readers)
	for _, profile := range profiles {
		go func(profile LLMTimeouts) {
			defer workers.Done()
			ready.Done()
			<-start
			for i := 0; i < iterations; i++ {
				SetLLMTimeouts(profile)
				if i%16 == 0 {
					runtime.Gosched()
				}
			}
		}(profile)
	}
	for reader := 0; reader < readers; reader++ {
		go func() {
			defer workers.Done()
			ready.Done()
			<-start
			for i := 0; i < iterations; i++ {
				snapshot := GetLLMTimeouts()
				if !accepted[snapshot] {
					invalid <- snapshot
					return
				}
				snapshot.HTTPClientTimeout = -time.Second
				snapshot.MaxRetries = -1
				if i%16 == 0 {
					runtime.Gosched()
				}
			}
		}()
	}
	ready.Wait()
	close(start)
	workers.Wait()
	close(invalid)
	for snapshot := range invalid {
		t.Errorf("observed unpublished or torn timeout profile: %+v", snapshot)
	}
	require.True(t, accepted[GetLLMTimeouts()], "final snapshot must be a complete published profile")
}

func TestLLMTimeoutPublication_DetachedValuesPreserveCallerInput(t *testing.T) {
	llmTimeoutPublicationFixture(t)
	input := llmTimeoutPublicationProfile(37*time.Second, 0)
	want := input
	SetLLMTimeouts(input)
	require.Equal(t, want, input)
	snapshot := GetLLMTimeouts()
	require.Equal(t, want, snapshot)
	snapshot.HTTPClientTimeout = -time.Second
	snapshot.MaxRetries = -1
	require.Equal(t, want, GetLLMTimeouts())
	require.Equal(t, want, input)
	input.FollowUpTimeout = time.Nanosecond
	input.MaxRetries = 99
	require.Equal(t, want, GetLLMTimeouts())
	SetLLMTimeouts(DefaultLLMTimeouts())
	require.Equal(t, -time.Second, snapshot.HTTPClientTimeout)
	require.Equal(t, want.PerCallTimeout, snapshot.PerCallTimeout)
}

func TestLLMTimeoutPublication_DefaultResolutionAndZeroValues(t *testing.T) {
	llmTimeoutPublicationFixture(t)
	defaults := LLMTimeouts{
		HTTPClientTimeout: 10 * time.Minute, SlotAcquisitionTimeout: 10 * time.Minute,
		PerCallTimeout: 10 * time.Minute, StreamingTimeout: 15 * time.Minute,
		RetryBackoffBase: time.Second, RetryBackoffMax: 30 * time.Second, MaxRetries: 3,
		RateLimitDelay: 600 * time.Millisecond, ArticulationTimeout: 5 * time.Minute,
		FollowUpTimeout: 5 * time.Minute,
	}
	require.Equal(t, defaults, DefaultLLMTimeouts())
	zero := 0
	noRetries := defaults
	noRetries.MaxRetries = 0
	override := defaults
	override.PerCallTimeout = 17 * time.Second
	cases := []struct {
		name   string
		config *LLMTimeoutsConfig
		want   LLMTimeouts
	}{
		{"absent", nil, defaults},
		{"empty", &LLMTimeoutsConfig{}, defaults},
		{"default", &LLMTimeoutsConfig{Profile: "default"}, defaults},
		{"fast", &LLMTimeoutsConfig{Profile: "fast"}, FastLLMTimeouts()},
		{"aggressive", &LLMTimeoutsConfig{Profile: "aggressive"}, AggressiveLLMTimeouts()},
		{"explicit_zero_retries", &LLMTimeoutsConfig{MaxRetries: &zero}, noRetries},
		{"partial_override", &LLMTimeoutsConfig{PerCallTimeout: "17s"}, override},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var before LLMTimeoutsConfig
			if tc.config != nil {
				before = *tc.config
			}
			resolved, err := tc.config.Resolve()
			require.NoError(t, err)
			require.Equal(t, tc.want, resolved)
			SetLLMTimeouts(resolved)
			require.Equal(t, tc.want, GetLLMTimeouts())
			require.Equal(t, tc.want, resolved)
			if tc.config != nil {
				require.Equal(t, before, *tc.config)
			}
			require.Zero(t, zero)
		})
	}
	SetLLMTimeouts(LLMTimeouts{})
	require.Equal(t, LLMTimeouts{}, GetLLMTimeouts())
	SetLLMTimeouts(defaults)
	require.Equal(t, defaults, GetLLMTimeouts())
}

func llmTimeoutPublicationFixture(t *testing.T) {
	t.Helper()
	before := GetLLMTimeouts()
	t.Cleanup(func() { SetLLMTimeouts(before) })
}

func llmTimeoutPublicationProfile(base time.Duration, retries int) LLMTimeouts {
	return LLMTimeouts{
		HTTPClientTimeout: base, SlotAcquisitionTimeout: base + time.Second,
		PerCallTimeout: base + 2*time.Second, StreamingTimeout: base + 3*time.Second,
		RetryBackoffBase: base + 4*time.Second, RetryBackoffMax: base + 5*time.Second,
		MaxRetries: retries, RateLimitDelay: base + 6*time.Second,
		ArticulationTimeout: base + 7*time.Second, FollowUpTimeout: base + 8*time.Second,
	}
}
