//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type apiKeyCreateRepoStub struct {
	apiKeyRepoStub
	created   *APIKey
	createErr error
}

func (s *apiKeyCreateRepoStub) Create(_ context.Context, key *APIKey) error {
	if s.createErr != nil {
		return s.createErr
	}
	clone := *key
	s.created = &clone
	return nil
}

type apiKeyGroupRepoStub struct {
	groupRepoNoop
	getByIDGroup     *Group
	listActiveGroups []Group
}

func (s *apiKeyGroupRepoStub) GetByID(_ context.Context, _ int64) (*Group, error) {
	if s.getByIDGroup == nil {
		return nil, ErrGroupNotFound
	}
	clone := *s.getByIDGroup
	return &clone, nil
}

func (s *apiKeyGroupRepoStub) ListActive(context.Context) ([]Group, error) {
	out := make([]Group, len(s.listActiveGroups))
	copy(out, s.listActiveGroups)
	return out, nil
}

func TestAPIKeyServiceCreate_AllowsExclusiveSubscriptionTypeGroupWhenUserIsAuthorized(t *testing.T) {
	repo := &apiKeyCreateRepoStub{}
	userRepo := &mockUserRepo{
		getByIDUser: &User{ID: 1, AllowedGroups: []int64{10}, Status: StatusActive},
	}
	groupRepo := &apiKeyGroupRepoStub{
		getByIDGroup: &Group{
			ID:               10,
			Name:             "exclusive-sub",
			IsExclusive:      true,
			Status:           StatusActive,
			SubscriptionType: SubscriptionTypeSubscription,
		},
	}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, userSubRepoNoop{}, nil, nil, &config.Config{})

	groupID := int64(10)
	key, err := svc.Create(context.Background(), 1, CreateAPIKeyRequest{
		Name:    "test",
		GroupID: &groupID,
	})
	require.NoError(t, err)
	require.NotNil(t, key.GroupID)
	require.Equal(t, int64(10), *key.GroupID)
	require.NotNil(t, repo.created)
	require.NotNil(t, repo.created.GroupID)
	require.Equal(t, int64(10), *repo.created.GroupID)
}

func TestAPIKeyServiceCreate_RejectsExclusiveGroupWithoutAuthorization(t *testing.T) {
	repo := &apiKeyCreateRepoStub{}
	userRepo := &mockUserRepo{
		getByIDUser: &User{ID: 1, AllowedGroups: nil, Status: StatusActive},
	}
	groupRepo := &apiKeyGroupRepoStub{
		getByIDGroup: &Group{
			ID:               10,
			Name:             "exclusive-sub",
			IsExclusive:      true,
			Status:           StatusActive,
			SubscriptionType: SubscriptionTypeSubscription,
		},
	}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, userSubRepoNoop{}, nil, nil, &config.Config{})

	groupID := int64(10)
	_, err := svc.Create(context.Background(), 1, CreateAPIKeyRequest{
		Name:    "test",
		GroupID: &groupID,
	})
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	require.Nil(t, repo.created)
}

func TestAPIKeyServiceGetAvailableGroups_IgnoresSubscriptionType(t *testing.T) {
	repo := &apiKeyCreateRepoStub{}
	userRepo := &mockUserRepo{
		getByIDUser: &User{ID: 1, AllowedGroups: []int64{2}, Status: StatusActive},
	}
	groupRepo := &apiKeyGroupRepoStub{
		listActiveGroups: []Group{
			{ID: 1, Name: "public-sub", IsExclusive: false, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
			{ID: 2, Name: "exclusive-sub", IsExclusive: true, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
			{ID: 3, Name: "exclusive-standard", IsExclusive: true, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
		},
	}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, nil, nil, nil, &config.Config{})

	groups, err := svc.GetAvailableGroups(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Equal(t, int64(1), groups[0].ID)
	require.Equal(t, int64(2), groups[1].ID)
}
