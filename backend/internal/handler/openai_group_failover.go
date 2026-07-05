package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

type openAIGroupFailoverRoute struct {
	originAPIKey *service.APIKey
	apiKey       *service.APIKey
	originGroup  int64
	routedGroup  int64
	reason       string
	usingBackup  bool
}

func newOpenAIGroupFailoverRoute(apiKey *service.APIKey) *openAIGroupFailoverRoute {
	groupID := apiKeyGroupIDValue(apiKey)
	return &openAIGroupFailoverRoute{
		originAPIKey: apiKey,
		apiKey:       apiKey,
		originGroup:  groupID,
		routedGroup:  groupID,
	}
}

func (h *OpenAIGatewayHandler) resolveInitialOpenAIGroupFailoverRoute(ctx context.Context, apiKey *service.APIKey, reqLog *zap.Logger) *openAIGroupFailoverRoute {
	route := newOpenAIGroupFailoverRoute(apiKey)
	useBackup, reason := h.gatewayService.ShouldUseOpenAIBackupGroup(apiKey)
	if useBackup {
		_ = h.activateOpenAIBackupGroup(ctx, route, reason, reqLog)
	}
	return route
}

func (h *OpenAIGatewayHandler) tryOpenAIBackupGroupAfterFailure(ctx context.Context, route *openAIGroupFailoverRoute, reason string, reqLog *zap.Logger) bool {
	if route == nil || route.usingBackup {
		return false
	}
	opened, failoverReason := h.gatewayService.RecordOpenAIGroupFailoverFailure(route.originAPIKey, reason)
	if !opened {
		return false
	}
	return h.activateOpenAIBackupGroup(ctx, route, failoverReason, reqLog)
}

func (h *OpenAIGatewayHandler) recordOpenAIGroupFailoverSuccess(route *openAIGroupFailoverRoute) {
	if route == nil || route.usingBackup {
		return
	}
	h.gatewayService.RecordOpenAIGroupFailoverSuccess(route.originAPIKey)
}

func (h *OpenAIGatewayHandler) activateOpenAIBackupGroup(ctx context.Context, route *openAIGroupFailoverRoute, reason string, reqLog *zap.Logger) bool {
	if h == nil || h.apiKeyService == nil || route == nil || !service.IsOpenAIBackupFailoverConfigured(route.originAPIKey) {
		return false
	}
	backupGroupID := route.originAPIKey.Group.BackupGroupID
	if backupGroupID == nil || *backupGroupID <= 0 {
		return false
	}
	backupGroup, err := h.apiKeyService.GetGroupByID(ctx, *backupGroupID)
	if err != nil {
		if reqLog != nil {
			reqLog.Warn("openai.group_failover_backup_group_load_failed",
				zap.Int64("origin_group_id", route.originGroup),
				zap.Int64("backup_group_id", *backupGroupID),
				zap.Error(err),
			)
		}
		return false
	}
	if backupGroup == nil || backupGroup.Status != service.StatusActive || backupGroup.Platform != service.PlatformOpenAI ||
		backupGroup.BackupFailoverEnabled || backupGroup.BackupGroupID != nil {
		if reqLog != nil {
			reqLog.Warn("openai.group_failover_backup_group_invalid",
				zap.Int64("origin_group_id", route.originGroup),
				zap.Int64("backup_group_id", *backupGroupID),
			)
		}
		return false
	}
	route.apiKey = cloneAPIKeyWithGroup(route.originAPIKey, backupGroup)
	route.routedGroup = backupGroup.ID
	route.reason = reason
	route.usingBackup = true
	if reqLog != nil {
		reqLog.Warn("openai.group_failover_backup_group_selected",
			zap.Int64("origin_group_id", route.originGroup),
			zap.Int64("backup_group_id", backupGroup.ID),
			zap.String("reason", reason),
		)
	}
	return true
}

func (r *openAIGroupFailoverRoute) APIKey() *service.APIKey {
	if r == nil || r.apiKey == nil {
		return nil
	}
	return r.apiKey
}

func (r *openAIGroupFailoverRoute) OriginGroupID() *int64 {
	if r == nil || r.originGroup <= 0 {
		return nil
	}
	value := r.originGroup
	return &value
}

func (r *openAIGroupFailoverRoute) RoutedGroupID() *int64 {
	if r == nil || r.routedGroup <= 0 {
		return nil
	}
	value := r.routedGroup
	return &value
}

func (r *openAIGroupFailoverRoute) FailoverReason() *string {
	if r == nil || r.reason == "" {
		return nil
	}
	value := r.reason
	return &value
}

func apiKeyGroupIDValue(apiKey *service.APIKey) int64 {
	if apiKey == nil {
		return 0
	}
	if apiKey.GroupID != nil && *apiKey.GroupID > 0 {
		return *apiKey.GroupID
	}
	if apiKey.Group != nil && apiKey.Group.ID > 0 {
		return apiKey.Group.ID
	}
	return 0
}
