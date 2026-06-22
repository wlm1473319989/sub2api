//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminServiceAdminUpdateAPIKeyGroupID_PublicSubscriptionTypeGroupDoesNotRequireSubscription(t *testing.T) {
	existing := &APIKey{ID: 1, UserID: 42, Key: "sk-test", GroupID: nil}
	apiKeyRepo := &apiKeyRepoStubForGroupUpdate{key: existing}
	groupRepo := &groupRepoStubForGroupUpdate{
		group: &Group{
			ID:               10,
			Name:             "Sub",
			Status:           StatusActive,
			IsExclusive:      false,
		},
	}
	userRepo := &userRepoStubForGroupUpdate{}
	svc := &adminServiceImpl{apiKeyRepo: apiKeyRepo, groupRepo: groupRepo, userRepo: userRepo}

	got, err := svc.AdminUpdateAPIKeyGroupID(context.Background(), 1, int64Ptr(10))
	require.NoError(t, err)
	require.NotNil(t, got.APIKey.GroupID)
	require.Equal(t, int64(10), *got.APIKey.GroupID)
	require.False(t, userRepo.addGroupCalled)
}

func TestAdminServiceAdminUpdateAPIKeyGroupID_ExclusiveSubscriptionTypeGroupAutoGrantsAllowedGroup(t *testing.T) {
	existing := &APIKey{ID: 1, UserID: 42, Key: "sk-test", GroupID: nil}
	apiKeyRepo := &apiKeyRepoStubForGroupUpdate{key: existing}
	groupRepo := &groupRepoStubForGroupUpdate{
		group: &Group{
			ID:               10,
			Name:             "Sub",
			Status:           StatusActive,
			IsExclusive:      true,
		},
	}
	userRepo := &userRepoStubForGroupUpdate{}
	svc := &adminServiceImpl{apiKeyRepo: apiKeyRepo, groupRepo: groupRepo, userRepo: userRepo}

	got, err := svc.AdminUpdateAPIKeyGroupID(context.Background(), 1, int64Ptr(10))
	require.NoError(t, err)
	require.NotNil(t, got.APIKey.GroupID)
	require.Equal(t, int64(10), *got.APIKey.GroupID)
	require.True(t, userRepo.addGroupCalled)
	require.Equal(t, int64(42), userRepo.addedUserID)
	require.Equal(t, int64(10), userRepo.addedGroupID)
	require.True(t, got.AutoGrantedGroupAccess)
	require.NotNil(t, got.GrantedGroupID)
	require.Equal(t, int64(10), *got.GrantedGroupID)
	require.Equal(t, "Sub", got.GrantedGroupName)
}
