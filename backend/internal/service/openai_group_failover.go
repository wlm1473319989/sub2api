package service

import (
	"math"
	"sync"
	"time"
)

const (
	OpenAIGroupFailoverReasonCircuitOpen      = "circuit_open"
	OpenAIGroupFailoverReasonFailureThreshold = "failure_threshold"
	OpenAIGroupFailoverReasonNoCapacity       = "no_capacity"
)

const (
	openAIGroupFailoverStateClosed   = "closed"
	openAIGroupFailoverStateOpen     = "open"
	openAIGroupFailoverStateHalfOpen = "half_open"
)

type openAIGroupFailoverState struct {
	mu                sync.Mutex
	state             string
	failures          []time.Time
	openUntil         time.Time
	openCount         int
	halfOpenSuccesses int
}

func IsOpenAIBackupFailoverConfigured(apiKey *APIKey) bool {
	if apiKey == nil || !apiKey.AllowPaidFailover || apiKey.Group == nil {
		return false
	}
	group := apiKey.Group
	return group.Platform == PlatformOpenAI &&
		group.Status == StatusActive &&
		group.BackupFailoverEnabled &&
		group.BackupGroupID != nil &&
		*group.BackupGroupID > 0
}

func (s *OpenAIGatewayService) ShouldUseOpenAIBackupGroup(apiKey *APIKey) (bool, string) {
	if s == nil || !IsOpenAIBackupFailoverConfigured(apiKey) {
		return false, ""
	}
	group := apiKey.Group
	state := s.getOpenAIGroupFailoverState(group.ID)
	cfg := NormalizeGroupBackupFailoverConfig(group.BackupFailoverConfig)
	now := time.Now()

	state.mu.Lock()
	defer state.mu.Unlock()

	switch state.state {
	case openAIGroupFailoverStateOpen:
		if now.Before(state.openUntil) {
			return true, OpenAIGroupFailoverReasonCircuitOpen
		}
		state.state = openAIGroupFailoverStateHalfOpen
		state.halfOpenSuccesses = 0
		return false, ""
	case openAIGroupFailoverStateHalfOpen:
		return false, ""
	default:
		state.state = openAIGroupFailoverStateClosed
		state.failures = pruneOpenAIGroupFailoverFailures(state.failures, now, time.Duration(cfg.FailureWindowSeconds)*time.Second)
		return false, ""
	}
}

func (s *OpenAIGatewayService) RecordOpenAIGroupFailoverFailure(apiKey *APIKey, reason string) (bool, string) {
	if s == nil || !IsOpenAIBackupFailoverConfigured(apiKey) {
		return false, ""
	}
	group := apiKey.Group
	state := s.getOpenAIGroupFailoverState(group.ID)
	cfg := NormalizeGroupBackupFailoverConfig(group.BackupFailoverConfig)
	now := time.Now()

	state.mu.Lock()
	defer state.mu.Unlock()

	switch state.state {
	case openAIGroupFailoverStateOpen:
		if now.Before(state.openUntil) {
			return true, OpenAIGroupFailoverReasonCircuitOpen
		}
		state.state = openAIGroupFailoverStateHalfOpen
		state.halfOpenSuccesses = 0
	case openAIGroupFailoverStateHalfOpen:
		openOpenAIGroupFailoverCircuit(state, cfg, now)
		return true, coalesceOpenAIGroupFailoverReason(reason, OpenAIGroupFailoverReasonFailureThreshold)
	default:
		state.state = openAIGroupFailoverStateClosed
	}

	window := time.Duration(cfg.FailureWindowSeconds) * time.Second
	state.failures = pruneOpenAIGroupFailoverFailures(state.failures, now, window)
	state.failures = append(state.failures, now)
	if len(state.failures) >= cfg.FailureThreshold {
		openOpenAIGroupFailoverCircuit(state, cfg, now)
		return true, coalesceOpenAIGroupFailoverReason(reason, OpenAIGroupFailoverReasonFailureThreshold)
	}
	return false, ""
}

func (s *OpenAIGatewayService) RecordOpenAIGroupFailoverSuccess(apiKey *APIKey) {
	if s == nil || !IsOpenAIBackupFailoverConfigured(apiKey) {
		return
	}
	group := apiKey.Group
	state := s.getOpenAIGroupFailoverState(group.ID)
	cfg := NormalizeGroupBackupFailoverConfig(group.BackupFailoverConfig)

	state.mu.Lock()
	defer state.mu.Unlock()

	switch state.state {
	case openAIGroupFailoverStateHalfOpen:
		state.halfOpenSuccesses++
		if state.halfOpenSuccesses >= cfg.HalfOpenSuccessThreshold {
			closeOpenAIGroupFailoverCircuit(state)
		}
	case openAIGroupFailoverStateOpen:
		if !time.Now().Before(state.openUntil) {
			state.state = openAIGroupFailoverStateHalfOpen
			state.halfOpenSuccesses = 1
			if state.halfOpenSuccesses >= cfg.HalfOpenSuccessThreshold {
				closeOpenAIGroupFailoverCircuit(state)
			}
		}
	default:
		closeOpenAIGroupFailoverCircuit(state)
	}
}

func (s *OpenAIGatewayService) ResetOpenAIGroupFailoverState(groupID int64) {
	if s == nil || groupID <= 0 {
		return
	}
	s.openaiGroupFailoverStates.Delete(groupID)
}

func (s *OpenAIGatewayService) getOpenAIGroupFailoverState(groupID int64) *openAIGroupFailoverState {
	if groupID <= 0 {
		return &openAIGroupFailoverState{state: openAIGroupFailoverStateClosed}
	}
	if value, ok := s.openaiGroupFailoverStates.Load(groupID); ok {
		if state, ok := value.(*openAIGroupFailoverState); ok && state != nil {
			return state
		}
	}
	state := &openAIGroupFailoverState{state: openAIGroupFailoverStateClosed}
	actual, _ := s.openaiGroupFailoverStates.LoadOrStore(groupID, state)
	if stored, ok := actual.(*openAIGroupFailoverState); ok && stored != nil {
		return stored
	}
	return state
}

func pruneOpenAIGroupFailoverFailures(failures []time.Time, now time.Time, window time.Duration) []time.Time {
	if len(failures) == 0 || window <= 0 {
		return nil
	}
	cutoff := now.Add(-window)
	idx := 0
	for idx < len(failures) && failures[idx].Before(cutoff) {
		idx++
	}
	if idx > 0 {
		copy(failures, failures[idx:])
		failures = failures[:len(failures)-idx]
	}
	return failures
}

func openOpenAIGroupFailoverCircuit(state *openAIGroupFailoverState, cfg GroupBackupFailoverConfig, now time.Time) {
	state.state = openAIGroupFailoverStateOpen
	state.failures = nil
	state.halfOpenSuccesses = 0
	state.openCount++
	cooldown := time.Duration(cfg.OpenCooldownSeconds) * time.Second
	if state.openCount > 1 && cfg.CooldownBackoffMultiplier > 1 {
		cooldown = time.Duration(float64(cooldown) * math.Pow(cfg.CooldownBackoffMultiplier, float64(state.openCount-1)))
	}
	maxCooldown := time.Duration(cfg.MaxOpenCooldownSeconds) * time.Second
	if maxCooldown > 0 && cooldown > maxCooldown {
		cooldown = maxCooldown
	}
	state.openUntil = now.Add(cooldown)
}

func closeOpenAIGroupFailoverCircuit(state *openAIGroupFailoverState) {
	state.state = openAIGroupFailoverStateClosed
	state.failures = nil
	state.openUntil = time.Time{}
	state.openCount = 0
	state.halfOpenSuccesses = 0
}

func coalesceOpenAIGroupFailoverReason(reason, fallback string) string {
	if reason != "" {
		return reason
	}
	return fallback
}
