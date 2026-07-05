package service

import (
	"testing"
	"time"
)

func TestOpenAIGroupFailoverCircuitOpensAfterThreshold(t *testing.T) {
	backupGroupID := int64(20)
	apiKey := testOpenAIBackupFailoverAPIKey(10, &backupGroupID, GroupBackupFailoverConfig{
		FailureWindowSeconds:      60,
		FailureThreshold:          3,
		OpenCooldownSeconds:       180,
		HalfOpenSuccessThreshold:  2,
		MaxOpenCooldownSeconds:    1800,
		CooldownBackoffMultiplier: 2,
	})
	svc := &OpenAIGatewayService{}

	if useBackup, reason := svc.ShouldUseOpenAIBackupGroup(apiKey); useBackup || reason != "" {
		t.Fatalf("expected closed circuit, got useBackup=%v reason=%q", useBackup, reason)
	}

	for i := 0; i < 2; i++ {
		opened, reason := svc.RecordOpenAIGroupFailoverFailure(apiKey, "")
		if opened || reason != "" {
			t.Fatalf("failure %d unexpectedly opened circuit: opened=%v reason=%q", i+1, opened, reason)
		}
	}

	opened, reason := svc.RecordOpenAIGroupFailoverFailure(apiKey, "")
	if !opened || reason != OpenAIGroupFailoverReasonFailureThreshold {
		t.Fatalf("expected threshold open, got opened=%v reason=%q", opened, reason)
	}

	useBackup, reason := svc.ShouldUseOpenAIBackupGroup(apiKey)
	if !useBackup || reason != OpenAIGroupFailoverReasonCircuitOpen {
		t.Fatalf("expected open circuit to use backup, got useBackup=%v reason=%q", useBackup, reason)
	}
}

func TestOpenAIGroupFailoverHalfOpenSuccessesCloseCircuit(t *testing.T) {
	backupGroupID := int64(20)
	apiKey := testOpenAIBackupFailoverAPIKey(10, &backupGroupID, GroupBackupFailoverConfig{
		FailureWindowSeconds:      60,
		FailureThreshold:          1,
		OpenCooldownSeconds:       180,
		HalfOpenSuccessThreshold:  2,
		MaxOpenCooldownSeconds:    1800,
		CooldownBackoffMultiplier: 2,
	})
	svc := &OpenAIGatewayService{}

	opened, _ := svc.RecordOpenAIGroupFailoverFailure(apiKey, "")
	if !opened {
		t.Fatal("expected first failure to open circuit")
	}

	state := svc.getOpenAIGroupFailoverState(10)
	state.mu.Lock()
	state.openUntil = time.Now().Add(-time.Second)
	state.mu.Unlock()

	useBackup, reason := svc.ShouldUseOpenAIBackupGroup(apiKey)
	if useBackup || reason != "" {
		t.Fatalf("expected half-open probe on primary, got useBackup=%v reason=%q", useBackup, reason)
	}

	svc.RecordOpenAIGroupFailoverSuccess(apiKey)
	state.mu.Lock()
	stateAfterOneSuccess := state.state
	state.mu.Unlock()
	if stateAfterOneSuccess != openAIGroupFailoverStateHalfOpen {
		t.Fatalf("expected circuit to remain half-open after one success, got %q", stateAfterOneSuccess)
	}

	svc.RecordOpenAIGroupFailoverSuccess(apiKey)
	state.mu.Lock()
	finalState := state.state
	state.mu.Unlock()
	if finalState != openAIGroupFailoverStateClosed {
		t.Fatalf("expected circuit to close after half-open successes, got %q", finalState)
	}
}

func TestOpenAIGroupFailoverRequiresPaidFailoverOptIn(t *testing.T) {
	backupGroupID := int64(20)
	apiKey := testOpenAIBackupFailoverAPIKey(10, &backupGroupID, GroupBackupFailoverConfig{
		FailureThreshold: 1,
	})
	apiKey.AllowPaidFailover = false
	svc := &OpenAIGatewayService{}

	if IsOpenAIBackupFailoverConfigured(apiKey) {
		t.Fatal("expected paid failover opt-in to be required")
	}
	opened, reason := svc.RecordOpenAIGroupFailoverFailure(apiKey, "")
	if opened || reason != "" {
		t.Fatalf("expected disabled failover, got opened=%v reason=%q", opened, reason)
	}
	useBackup, reason := svc.ShouldUseOpenAIBackupGroup(apiKey)
	if useBackup || reason != "" {
		t.Fatalf("expected disabled failover route, got useBackup=%v reason=%q", useBackup, reason)
	}
}

func testOpenAIBackupFailoverAPIKey(groupID int64, backupGroupID *int64, cfg GroupBackupFailoverConfig) *APIKey {
	return &APIKey{
		ID:                1,
		UserID:            1,
		GroupID:           &groupID,
		AllowPaidFailover: true,
		Group: &Group{
			ID:                    groupID,
			Platform:              PlatformOpenAI,
			Status:                StatusActive,
			BackupFailoverEnabled: true,
			BackupGroupID:         backupGroupID,
			BackupFailoverConfig:  cfg,
		},
	}
}
