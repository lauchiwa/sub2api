package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

var (
	ErrOpenAIReviveProbeNoResult = errors.New("openai revive probe returned no result")
	ErrOpenAIReviveProbeFailed   = errors.New("openai revive probe failed")
)

// OpenAIReviveCheckService periodically probes OpenAI OAuth accounts and attempts
// token refresh before/after health checks. It reuses existing AccountTestService
// and TokenRefreshService primitives so scheduled checks stay aligned with the
// admin refresh/test flows.
type OpenAIReviveCheckService struct {
	accountRepo         AccountRepository
	accountTestService  *AccountTestService
	tokenRefreshService *TokenRefreshService
	openaiOAuthService  *OpenAIOAuthService
	cfg                 *config.TokenRefreshConfig

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewOpenAIReviveCheckService(
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
	tokenRefreshService *TokenRefreshService,
	openaiOAuthService *OpenAIOAuthService,
	cfg *config.Config,
) *OpenAIReviveCheckService {
	var tokenCfg *config.TokenRefreshConfig
	if cfg != nil {
		tokenCfg = &cfg.TokenRefresh
	}
	return &OpenAIReviveCheckService{
		accountRepo:         accountRepo,
		accountTestService:  accountTestService,
		tokenRefreshService: tokenRefreshService,
		openaiOAuthService:  openaiOAuthService,
		cfg:                 tokenCfg,
		stopCh:              make(chan struct{}),
	}
}

func (s *OpenAIReviveCheckService) Start() {
	if s == nil || s.cfg == nil || !s.cfg.Enabled {
		return
	}
	if s.accountRepo == nil || s.accountTestService == nil || s.tokenRefreshService == nil || s.openaiOAuthService == nil {
		slog.Warn("openai_revive_check.service_not_started", "reason", "missing_dependency")
		return
	}

	s.wg.Add(1)
	go s.loop()
	slog.Info("openai_revive_check.service_started", "check_interval_minutes", s.cfg.CheckIntervalMinutes)
}

func (s *OpenAIReviveCheckService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *OpenAIReviveCheckService) loop() {
	defer s.wg.Done()

	interval := time.Duration(s.cfg.CheckIntervalMinutes) * time.Minute
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.process(context.Background())
	for {
		select {
		case <-ticker.C:
			s.process(context.Background())
		case <-s.stopCh:
			return
		}
	}
}

func (s *OpenAIReviveCheckService) process(ctx context.Context) {
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		slog.Warn("openai_revive_check.list_accounts_failed", "error", err)
		return
	}

	checked, refreshed, recovered, probeFailed, skipped := 0, 0, 0, 0, 0
	refreshWindow := time.Duration(s.cfg.RefreshBeforeExpiryHours * float64(time.Hour))
	refresher := NewOpenAITokenRefresher(s.openaiOAuthService, s.accountRepo)

	for i := range accounts {
		account := &accounts[i]
		if !shouldCheckOpenAIAccount(account) {
			skipped++
			continue
		}
		checked++

		if refresher.NeedsRefresh(account, refreshWindow) || shouldReviveOpenAIAccount(account) {
			if err := s.tokenRefreshService.refreshWithRetry(ctx, account, refresher, refresher, refreshWindow); err != nil {
				slog.Warn("openai_revive_check.refresh_failed", "account_id", account.ID, "error", err)
			} else {
				refreshed++
			}
		}

		if err := s.testAccount(ctx, account.ID); err != nil {
			probeFailed++
			slog.Debug("openai_revive_check.probe_failed", "account_id", account.ID, "error", err)
		} else if s.recoverAccountState(ctx, account) {
			recovered++
		}
	}

	slog.Info("openai_revive_check.cycle_completed", "total", len(accounts), "checked", checked, "refreshed", refreshed, "recovered", recovered, "probe_failed", probeFailed, "skipped", skipped)
}

func (s *OpenAIReviveCheckService) testAccount(ctx context.Context, accountID int64) error {
	result, err := s.accountTestService.RunTestBackground(ctx, accountID, "")
	if err != nil {
		return err
	}
	if result == nil {
		return ErrOpenAIReviveProbeNoResult
	}
	if result.Status != "success" {
		if result.ErrorMessage != "" {
			return errors.New(result.ErrorMessage)
		}
		return ErrOpenAIReviveProbeFailed
	}
	return nil
}

func shouldCheckOpenAIAccount(account *Account) bool {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return false
	}
	if account.Status == StatusDisabled || account.Status == StatusExpired {
		return false
	}
	return true
}

func (s *OpenAIReviveCheckService) recoverAccountState(ctx context.Context, account *Account) bool {
	if s == nil || s.accountRepo == nil || account == nil {
		return false
	}

	recovered := false
	if account.IsRateLimited() {
		if err := s.accountRepo.ClearRateLimit(ctx, account.ID); err != nil {
			slog.Warn("openai_revive_check.clear_rate_limit_failed", "account_id", account.ID, "error", err)
		} else {
			account.RateLimitResetAt = nil
			recovered = true
		}
	}

	if account.Status == StatusError {
		if err := s.accountRepo.ClearError(ctx, account.ID); err != nil {
			slog.Warn("openai_revive_check.clear_error_failed", "account_id", account.ID, "error", err)
		} else {
			account.Status = StatusActive
			account.ErrorMessage = ""
			recovered = true
		}
	}

	return recovered
}

func shouldReviveOpenAIAccount(account *Account) bool {
	if !shouldCheckOpenAIAccount(account) {
		return false
	}
	if account.IsRateLimited() {
		return true
	}
	if account.Status == StatusError {
		msg := strings.ToLower(account.ErrorMessage)
		return strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "authentication failed") || strings.Contains(msg, "token")
	}
	return false
}
