//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type openAIReviveCheckRepo struct {
	mockAccountRepoForGemini
	clearRateLimitIDs []int64
	clearErrorIDs     []int64
}

func (r *openAIReviveCheckRepo) ClearRateLimit(_ context.Context, id int64) error {
	r.clearRateLimitIDs = append(r.clearRateLimitIDs, id)
	return nil
}

func (r *openAIReviveCheckRepo) ClearError(_ context.Context, id int64) error {
	r.clearErrorIDs = append(r.clearErrorIDs, id)
	return nil
}

type openAIReviveCheckFailingUpstream struct{}

func (u *openAIReviveCheckFailingUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("probe still failing")),
	}, nil
}

func (u *openAIReviveCheckFailingUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestShouldCheckOpenAIAccountFiltersUnsafeAccounts(t *testing.T) {
	t.Parallel()

	require.False(t, shouldCheckOpenAIAccount(nil))
	require.False(t, shouldCheckOpenAIAccount(&Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive}))
	require.False(t, shouldCheckOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive}))
	require.False(t, shouldCheckOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusDisabled}))
	require.False(t, shouldCheckOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusExpired}))
	require.True(t, shouldCheckOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive}))
	require.True(t, shouldCheckOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, ErrorMessage: "Authentication failed (401)"}))
}

func TestShouldReviveOpenAIAccountOnlyTargetsRecoverableStates(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(time.Hour)
	require.True(t, shouldReviveOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, RateLimitResetAt: &future}))
	require.True(t, shouldReviveOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, ErrorMessage: "Authentication failed (401)"}))
	require.True(t, shouldReviveOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, ErrorMessage: "token expired"}))
	require.False(t, shouldReviveOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError, ErrorMessage: "quota exceeded"}))
	require.False(t, shouldReviveOpenAIAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusDisabled, RateLimitResetAt: &future}))
}

func TestOpenAIReviveCheckRecoverAccountStateClearsOnlyAfterSuccessfulProbe(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(time.Hour)
	account := &Account{
		ID:               88,
		Platform:         PlatformOpenAI,
		Type:             AccountTypeOAuth,
		Status:           StatusError,
		ErrorMessage:     "Authentication failed (401)",
		RateLimitResetAt: &future,
	}
	repo := &openAIReviveCheckRepo{}
	svc := &OpenAIReviveCheckService{accountRepo: repo}

	recovered := svc.recoverAccountState(context.Background(), account)

	require.True(t, recovered)
	require.Equal(t, []int64{88}, repo.clearRateLimitIDs)
	require.Equal(t, []int64{88}, repo.clearErrorIDs)
	require.Nil(t, account.RateLimitResetAt)
	require.Equal(t, StatusActive, account.Status)
	require.Empty(t, account.ErrorMessage)
}

func TestOpenAIReviveCheckTestAccountFailsWhenBackgroundProbeReturnsFailedStatus(t *testing.T) {
	t.Parallel()

	svc := &OpenAIReviveCheckService{
		accountTestService: &AccountTestService{
			accountRepo: &mockAccountRepoForGemini{accountsByID: map[int64]*Account{
				99: {
					ID:       99,
					Platform: PlatformOpenAI,
					Type:     AccountTypeOAuth,
					Status:   StatusError,
					Credentials: map[string]any{
						"access_token": "test-token",
					},
				},
			}},
			httpUpstream:        &openAIReviveCheckFailingUpstream{},
			cfg:                 &config.Config{},
			tlsFPProfileService: &TLSFingerprintProfileService{},
		},
	}

	err := svc.testAccount(context.Background(), 99)

	require.Error(t, err)
	require.ErrorContains(t, err, "probe still failing")
}
