package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type insufficientBalanceSettingRepoStub struct {
	values         map[string]string
	updates        map[string]string
	getMultipleErr error
}

func (s *insufficientBalanceSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *insufficientBalanceSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *insufficientBalanceSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *insufficientBalanceSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if s.getMultipleErr != nil {
		return nil, s.getMultipleErr
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *insufficientBalanceSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.updates = make(map[string]string, len(settings))
	for key, value := range settings {
		s.updates[key] = value
	}
	return nil
}

func (s *insufficientBalanceSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *insufficientBalanceSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestResolveInsufficientBalanceError_UsesCustomSettings(t *testing.T) {
	svc := NewSettingService(&insufficientBalanceSettingRepoStub{
		values: map[string]string{
			SettingKeyInsufficientBalanceErrorCustomEnabled: "true",
			SettingKeyInsufficientBalanceErrorCode:          " RECHARGE_REQUIRED ",
			SettingKeyInsufficientBalanceErrorMessage:       " Please recharge before retrying. ",
		},
	}, &config.Config{})

	code, message := ResolveInsufficientBalanceError(context.Background(), svc, DefaultInsufficientBalanceAuthCode, DefaultInsufficientBalanceAuthMessage)

	require.Equal(t, "RECHARGE_REQUIRED", code)
	require.Equal(t, "Please recharge before retrying.", message)
}

func TestResolveInsufficientBalanceError_FallsBackWhenDisabledOrUnavailable(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		svc := NewSettingService(&insufficientBalanceSettingRepoStub{
			values: map[string]string{
				SettingKeyInsufficientBalanceErrorCustomEnabled: "false",
				SettingKeyInsufficientBalanceErrorCode:          "RECHARGE_REQUIRED",
				SettingKeyInsufficientBalanceErrorMessage:       "Please recharge.",
			},
		}, &config.Config{})

		code, message := ResolveInsufficientBalanceError(context.Background(), svc, "DEFAULT_CODE", "default message")

		require.Equal(t, "DEFAULT_CODE", code)
		require.Equal(t, "default message", message)
	})

	t.Run("repo error", func(t *testing.T) {
		svc := NewSettingService(&insufficientBalanceSettingRepoStub{
			getMultipleErr: errors.New("db unavailable"),
		}, &config.Config{})

		code, message := ResolveInsufficientBalanceError(context.Background(), svc, "DEFAULT_CODE", "default message")

		require.Equal(t, "DEFAULT_CODE", code)
		require.Equal(t, "default message", message)
	})
}

func TestSettingService_UpdateSettings_WritesInsufficientBalanceErrorSettings(t *testing.T) {
	repo := &insufficientBalanceSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		InsufficientBalanceErrorCustomEnabled: true,
		InsufficientBalanceErrorCode:          " RECHARGE_REQUIRED ",
		InsufficientBalanceErrorMessage:       " Please recharge before retrying. ",
	})

	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyInsufficientBalanceErrorCustomEnabled])
	require.Equal(t, "RECHARGE_REQUIRED", repo.updates[SettingKeyInsufficientBalanceErrorCode])
	require.Equal(t, "Please recharge before retrying.", repo.updates[SettingKeyInsufficientBalanceErrorMessage])
}
