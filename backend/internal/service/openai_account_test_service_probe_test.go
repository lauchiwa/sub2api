//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIProbeAccountRepoStub struct {
	updatedID            int64
	updatedExtra         map[string]any
	updateCalls          int
	setRateLimitedCalls  int
	setErrorCalls        int
	rateLimitedID        int64
	rateLimitedResetAtOK bool
	errorID              int64
	errorMessage         string
}

func (s *openAIProbeAccountRepoStub) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	s.updatedID = id
	s.updateCalls++
	s.updatedExtra = make(map[string]any, len(updates))
	for k, v := range updates {
		s.updatedExtra[k] = v
	}
	return nil
}

func (s *openAIProbeAccountRepoStub) SetRateLimited(_ context.Context, id int64, _ time.Time) error {
	s.setRateLimitedCalls++
	s.rateLimitedID = id
	s.rateLimitedResetAtOK = true
	return nil
}

func (s *openAIProbeAccountRepoStub) SetError(_ context.Context, id int64, errorMsg string) error {
	s.setErrorCalls++
	s.errorID = id
	s.errorMessage = errorMsg
	return nil
}

func (s *openAIProbeAccountRepoStub) GetByID(context.Context, int64) (*Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) GetByIDs(context.Context, []int64) ([]*Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ExistsByID(context.Context, int64) (bool, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) GetByCRSAccountID(context.Context, string) (*Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) FindByExtraField(context.Context, string, any) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) List(context.Context, pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) Create(context.Context, *Account) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) Update(context.Context, *Account) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) Delete(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) UpdateLastUsed(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) SetSchedulable(context.Context, int64, bool) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) BindGroups(context.Context, int64, []int64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListSchedulable(context.Context) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListSchedulableByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListSchedulableByPlatforms(context.Context, []string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) SetModelRateLimit(context.Context, int64, string, time.Time) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ClearAntigravityQuotaScopes(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ClearModelRateLimits(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) IncrementQuotaUsed(context.Context, int64, float64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ResetQuotaUsed(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListByType(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListByStatus(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListByGroup(context.Context, int64) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListActive(context.Context) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ListByPlatform(context.Context, string) ([]Account, error) {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ClearRateLimit(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ClearError(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) SetOverloaded(context.Context, int64, time.Time) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	panic("unexpected")
}
func (s *openAIProbeAccountRepoStub) BulkUpdate(context.Context, []int64, AccountBulkUpdate) (int64, error) {
	panic("unexpected")
}

type openAIProbeFixedUpstream struct {
	resp *http.Response
}

func (u *openAIProbeFixedUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.resp, nil
}

func (u *openAIProbeFixedUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func newOpenAIProbeTestContext(ctx context.Context) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil).WithContext(ctx)
	return c, w
}

func TestAccountTestService_OpenAIOAuth403ProbePersistsCFSignals(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	repo := &openAIProbeAccountRepoStub{}
	upstream := &openAIProbeFixedUpstream{resp: &http.Response{
		StatusCode: http.StatusForbidden,
		Header: http.Header{
			"cf-mitigated":                       []string{"challenge"},
			"x-codex-primary-used-percent":       []string{"100"},
			"x-codex-primary-reset-after-seconds": []string{"60"},
		},
		Body: io.NopCloser(strings.NewReader(`{"error":"forbidden"}`)),
	}}
	service := NewAccountTestService(repo, nil, nil, upstream, &config.Config{}, nil)
	account := &Account{
		ID:       88,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "***",
		},
	}
	ctx, _ := newOpenAIProbeTestContext(context.Background())

	err := service.testOpenAIAccountConnection(ctx, account, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "API returned 403")

	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, int64(88), repo.updatedID)
	require.Equal(t, PrivacyModeCFBlocked, repo.updatedExtra["privacy_mode"])
	require.Equal(t, http.StatusForbidden, repo.updatedExtra["probe_last_status"])
	require.Equal(t, true, repo.updatedExtra["probe_cf_blocked"])
	require.Equal(t, PrivacyModeCFBlocked, account.Extra["privacy_mode"])
	require.True(t, repo.setRateLimitedCalls <= 1)
	if repo.setRateLimitedCalls == 1 {
		require.Equal(t, int64(88), repo.rateLimitedID)
	}
	require.Zero(t, repo.setErrorCalls)
}

func TestAccountTestService_OpenAIOAuth401MarksPermanentError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	repo := &openAIProbeAccountRepoStub{}
	upstream := &openAIProbeFixedUpstream{resp: &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(`{"error":"bad token"}`)),
	}}
	service := NewAccountTestService(repo, nil, nil, upstream, &config.Config{}, nil)
	account := &Account{
		ID:       89,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "***",
		},
	}
	ctx, _ := newOpenAIProbeTestContext(context.Background())

	err := service.testOpenAIAccountConnection(ctx, account, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "API returned 401")

	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, int64(89), repo.errorID)
	require.Contains(t, repo.errorMessage, "Authentication failed (401)")
	require.Equal(t, 0, repo.updateCalls)
	}
