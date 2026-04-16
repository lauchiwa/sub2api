//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

type openAIPrivacyAccountRepoStub struct {
	updatedID     int64
	updatedExtra  map[string]any
	updateCalls   int
}

func (s *openAIPrivacyAccountRepoStub) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	s.updatedID = id
	s.updateCalls++
	s.updatedExtra = make(map[string]any, len(updates))
	for k, v := range updates {
		s.updatedExtra[k] = v
	}
	return nil
}

func (s *openAIPrivacyAccountRepoStub) GetByID(context.Context, int64) (*Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) GetByIDs(context.Context, []int64) ([]*Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ExistsByID(context.Context, int64) (bool, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) GetByCRSAccountID(context.Context, string) (*Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) FindByExtraField(context.Context, string, any) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) List(context.Context, pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) Create(context.Context, *Account) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) Update(context.Context, *Account) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) Delete(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) UpdateLastUsed(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) SetSchedulable(context.Context, int64, bool) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) BindGroups(context.Context, int64, []int64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListSchedulable(context.Context) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListSchedulableByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListSchedulableByPlatforms(context.Context, []string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) SetModelRateLimit(context.Context, int64, string, time.Time) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ClearAntigravityQuotaScopes(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ClearModelRateLimits(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) IncrementQuotaUsed(context.Context, int64, float64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ResetQuotaUsed(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListByType(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListByStatus(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListByGroup(context.Context, int64) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListActive(context.Context) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ListByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) SetRateLimited(context.Context, int64, time.Time) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ClearRateLimit(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) SetError(context.Context, int64, string) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ClearError(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) SetOverloaded(context.Context, int64, time.Time) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIPrivacyAccountRepoStub) BulkUpdate(context.Context, []int64, AccountBulkUpdate) (int64, error) {
	panic("unexpected")
}

func TestTokenRefreshServiceEnsureOpenAIPrivacy_PersistsProbeModeWhenFactoryFails(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		TokenRefresh: config.TokenRefreshConfig{
			MaxRetries:          1,
			RetryBackoffSeconds: 0,
		},
	}
	repo := &openAIPrivacyAccountRepoStub{}
	svc := NewTokenRefreshService(repo, nil, nil, nil, nil, nil, nil, cfg, nil)
	privacyCalls := 0
	svc.SetPrivacyDeps(func(proxyURL string) (*req.Client, error) {
		privacyCalls++
		return nil, errors.New("factory failed")
	}, nil)

	account := &Account{
		ID:       202,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "***",
		},
		Extra: map[string]any{
			"privacy_mode": PrivacyModeFailed,
		},
	}

	svc.ensureOpenAIPrivacy(context.Background(), account)

	require.Equal(t, 1, privacyCalls)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, int64(202), repo.updatedID)
	require.Equal(t, PrivacyModeFailed, repo.updatedExtra["privacy_mode"])
	require.Equal(t, PrivacyModeFailed, account.Extra["privacy_mode"])
}

func TestExtractOpenAICodexProbeUpdates_StoresProbeSignalsFrom403Response(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header: http.Header{
			"cf-mitigated": []string{"challenge"},
		},
	}

	updates, err := extractOpenAICodexProbeUpdates(resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "openai codex probe returned status 403")
	require.Equal(t, PrivacyModeCFBlocked, updates["privacy_mode"])
	require.Equal(t, http.StatusForbidden, updates["probe_last_status"])
	require.Equal(t, true, updates["probe_cf_blocked"])
}
