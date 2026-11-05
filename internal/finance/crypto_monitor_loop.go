package finance

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
)

func (a *App) cryptoLoop(ctx context.Context) {
	var workers sync.WaitGroup
	defer func() {
		a.crypto.mu.Lock()
		for _, run := range a.crypto.inflight {
			run.cancel()
		}
		a.crypto.mu.Unlock()
		workers.Wait()
	}()
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			rows, e := a.h.DB.Query(`SELECT user_id FROM finance_crypto_monitors WHERE enabled=1 AND next_run<>'' AND next_run<=? ORDER BY next_run LIMIT 32`, time.Now().UTC().Format(time.RFC3339Nano))
			if e != nil {
				continue
			}
			var users []string
			for rows.Next() {
				var user string
				if rows.Scan(&user) == nil {
					users = append(users, user)
				}
			}
			rows.Close()
			for _, user := range users {
				workers.Add(1)
				go func(user string) { defer workers.Done(); a.cryptoRun(ctx, user) }(user)
			}
		}
	}
}

func (a *App) cryptoRunCurrent(user string, revision int64, keyID string) bool {
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	v, e := a.cryptoLoad(user)
	if e != nil || !a.cryptoActive(user) || !v.State.Config.Enabled || v.State.Revision != revision || v.KeyID != keyID {
		return false
	}
	current, e := a.cryptoKeyID(user)
	return e == nil && current == keyID
}

func (a *App) cryptoStart(ctx context.Context, user string) (cryptoEnvelope, string, context.Context, context.CancelFunc, error) {
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	v, e := a.cryptoLoad(user)
	if e != nil {
		return v, "", nil, nil, e
	}
	if !v.State.Config.Enabled || v.ConsentedAt == "" || len(a.crypto.inflight) >= 2 {
		return v, "", nil, nil, errors.New("Monitor not available for a new check")
	}
	if _, ok := a.crypto.inflight[user]; ok {
		return v, "", nil, nil, errors.New("Check already running")
	}
	now := time.Now().UTC()
	due, e := time.Parse(time.RFC3339Nano, v.State.NextRun)
	if e != nil || now.Before(due) || cryptoPending(v.State, now) {
		return v, "", nil, nil, errors.New("Monitor is not due")
	}
	keyID, keyErr := a.cryptoKeyID(user)
	if !a.cryptoActive(user) || keyErr != nil || keyID != v.KeyID {
		old := v.State.Revision
		v.State.Revision++
		v.State.Config.Enabled = false
		v.State.Status = "paused"
		v.State.NextRun = ""
		v.State.LastError = "The account, DeepSeek key or model changed; review monitoring consent again"
		cryptoExpire(&v.State, now, true)
		_ = a.cryptoSave(v, old)
		return v, "", nil, nil, errors.New(v.State.LastError)
	}
	if cryptoAwaitingReceipt(v.State) {
		old := v.State.Revision
		v.State.Revision++
		v.State.Status = "receipt"
		v.State.LastError = cryptoReceiptWaitMessage
		v.State.NextRun = now.Add(time.Duration(v.State.Config.IntervalMinutes) * time.Minute).Format(time.RFC3339Nano)
		_ = a.cryptoSave(v, old)
		return v, "", nil, nil, errors.New(cryptoReceiptWaitMessage)
	}
	cryptoUsageRefresh(&v.State, now)
	if v.State.Usage.Day > now.Format("2006-01-02") {
		return v, "", nil, nil, errors.New("Clock moved behind the recorded usage day")
	}
	if v.State.Usage.Analyses >= v.State.Config.MaxAnalysesPerDay || cryptoMicros(v.State.Usage.ReservedSpendUSD)+cryptoReservationMicros > cryptoMicros(v.State.Config.MaxSpendUSD) {
		old := v.State.Revision
		v.State.Revision++
		v.State.Status = "limit"
		v.State.LastError = "The daily analysis or estimated API cost cap has been reached"
		v.State.NextRun = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 5, 0, 0, time.UTC).Format(time.RFC3339Nano)
		_ = a.cryptoSave(v, old)
		return v, "", nil, nil, errors.New(v.State.LastError)
	}
	model, key, e := a.aiCredential(user, "deepseek")
	if e != nil || model != "deepseek-flash" {
		return v, "", nil, nil, errors.New("An available deepseek-flash key is required")
	}
	if !a.aiBegin(user, now) {
		return v, "", nil, nil, errors.New("Another AI request or rate limit is active")
	}
	old := v.State.Revision
	v.State.Revision++
	cryptoExpire(&v.State, now, false)
	v.State.Status = "checking"
	v.State.LastError = ""
	v.State.LastRun = now.Format(time.RFC3339Nano)
	v.State.NextRun = now.Add(time.Duration(v.State.Config.IntervalMinutes) * time.Minute).Format(time.RFC3339Nano)
	v.State.Usage.Analyses++
	v.State.Usage.ReservedSpendUSD = cryptoUSD(cryptoMicros(v.State.Usage.ReservedSpendUSD) + cryptoReservationMicros)
	cryptoUsageRefresh(&v.State, now)
	v.Attempts = append(v.Attempts, cryptoAttempt{At: v.State.LastRun, ReservedMicros: cryptoReservationMicros, Outcome: "reserved"})
	if len(v.Attempts) > 32 {
		v.Attempts = v.Attempts[len(v.Attempts)-32:]
	}
	if e = a.cryptoSave(v, old); e != nil {
		a.aiEnd(user)
		return v, "", nil, nil, e
	}
	runCtx, cancel := context.WithTimeout(ctx, 150*time.Second)
	a.crypto.inflight[user] = cryptoFlight{revision: v.State.Revision, cancel: cancel}
	return v, key, runCtx, cancel, nil
}

func (a *App) cryptoRun(ctx context.Context, user string) {
	v, key, runCtx, cancel, e := a.cryptoStart(ctx, user)
	if e != nil {
		return
	}
	defer a.aiEnd(user)
	defer cancel()
	defer func() {
		a.crypto.mu.Lock()
		if f, ok := a.crypto.inflight[user]; ok && f.revision == v.State.Revision {
			delete(a.crypto.inflight, user)
		}
		a.crypto.mu.Unlock()
	}()
	defer a.cryptoFinishInterrupted(user, v.State.Revision, v.KeyID)
	current := func() bool { return runCtx.Err() == nil && a.cryptoRunCurrent(user, v.State.Revision, v.KeyID) }
	market, e := a.cryptoMarketRead(runCtx)
	var wallet cryptoWalletSnapshot
	var evidence []cryptoEvidence
	var decision cryptoDecision
	var proposal *cryptoProposal
	if e == nil && current() {
		wallet, e = a.cryptoWalletSnapshot(runCtx, v.State.Config.WalletAddress)
	} else if e == nil {
		e = errors.New("Monitoring changed before balances were retrieved")
	}
	if e == nil {
		evidence = []cryptoEvidence{{ID: "market", Kind: "market", Title: "Base ETH/USDC market snapshot", URL: market.SourceURL, Snippet: "Observed pool price and liquidity; an indicative snapshot, not an executable quote", FetchedAt: market.FetchedAt}}
	}
	for _, kind := range []string{"news", "indexed-social"} {
		if e != nil {
			break
		}
		if !current() {
			e = errors.New("Monitoring changed before paid evidence collection")
			break
		}
		var found []cryptoEvidence
		found, e = a.cryptoSearch(runCtx, key, kind, time.Now().UTC())
		if e == nil {
			evidence = append(evidence, found...)
		}
	}
	if e == nil {
		var input []byte
		input, e = cryptoModelInput(v.State.Config, market, wallet, evidence)
		if e == nil {
			if current() {
				decision, e = a.cryptoModelCall(runCtx, key, input)
			} else {
				e = errors.New("Monitoring changed before analysis")
			}
		}
	}
	if e == nil {
		proposal, e = cryptoValidateDecision(decision, v.State.Config, market, wallet, evidence, time.Now().UTC())
	}
	if !a.cryptoRunCurrent(user, v.State.Revision, v.KeyID) {
		return
	}
	notificationID := ""
	a.crypto.mu.Lock()
	fresh, loadErr := a.cryptoLoad(user)
	keyID, keyErr := a.cryptoKeyID(user)
	if loadErr == nil && keyErr == nil && keyID == v.KeyID && fresh.State.Config.Enabled && fresh.State.Revision == v.State.Revision && a.cryptoActive(user) {
		old := fresh.State.Revision
		fresh.State.Revision++
		if e != nil {
			fresh.State.Status = "error"
			fresh.State.LastError = e.Error()
			if len(fresh.Attempts) > 0 {
				fresh.Attempts[len(fresh.Attempts)-1].Outcome = "failed"
			}
		} else {
			fresh.State.Market = &market
			fresh.State.Wallet = &wallet
			fresh.State.Evidence = evidence
			fresh.State.LastAnalysis = decision.Rationale
			fresh.State.LastRisks = decision.Risks
			fresh.State.LastError = ""
			fresh.State.Status = "hold"
			if len(fresh.Attempts) > 0 {
				fresh.Attempts[len(fresh.Attempts)-1].Outcome = "completed"
			}
			if proposal != nil {
				// Preserve unresolved transaction receipts while keeping history bounded.
				for len(fresh.State.Proposals) >= 30 {
					removed := false
					for i, p := range fresh.State.Proposals {
						if p.Status != "submitted" && p.Status != "unknown" && p.Status != "pending" {
							fresh.State.Proposals = append(fresh.State.Proposals[:i], fresh.State.Proposals[i+1:]...)
							removed = true
							break
						}
					}
					if !removed {
						break
					}
				}
				if len(fresh.State.Proposals) < 30 {
					now := time.Now().UTC()
					proposal.ID = apphost.ID()
					proposal.Revision = fresh.State.Revision
					proposal.CreatedAt = now.Format(time.RFC3339Nano)
					proposal.ExpiresAt = now.Add(15 * time.Minute).Format(time.RFC3339Nano)
					fresh.State.Proposals = append(fresh.State.Proposals, *proposal)
					fresh.State.Status = "proposal"
					notificationID = proposal.ID
				} else {
					fresh.State.LastError = "Resolve outstanding transaction receipts before creating another proposal"
				}
			}
		}
		if a.cryptoSave(fresh, old) != nil {
			notificationID = ""
		}
	}
	a.crypto.mu.Unlock()
	if notificationID != "" {
		a.cryptoQueueNotification(user, notificationID)
	}
}

func (a *App) cryptoFinishInterrupted(user string, revision int64, keyID string) {
	a.crypto.mu.Lock()
	defer a.crypto.mu.Unlock()
	v, e := a.cryptoLoad(user)
	if e != nil || v.State.Revision != revision || v.State.Status != "checking" {
		return
	}
	old := v.State.Revision
	v.State.Revision++
	v.State.Status = "error"
	v.State.LastError = "The check was interrupted; its reserved estimate remains counted and it was not retried"
	current, e := a.cryptoKeyID(user)
	if e != nil || current != keyID || !a.cryptoActive(user) {
		v.State.Config.Enabled = false
		v.State.Status = "paused"
		v.State.NextRun = ""
		v.State.LastError = "The account, DeepSeek key or model changed; review monitor consent again"
		cryptoExpire(&v.State, time.Now(), true)
	}
	if len(v.Attempts) > 0 {
		v.Attempts[len(v.Attempts)-1].Outcome = "interrupted"
	}
	_ = a.cryptoSave(v, old)
}
