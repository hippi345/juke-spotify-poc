package evals

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// LiveCallBudget limits outbound Gemini/Spotify calls during JUKE_EVALS_LIVE runs.
type LiveCallBudget struct {
	cap   int
	count int
	mu    sync.Mutex
}

// DefaultLiveCallCap is above a full-suite live run (~212 calls/arm) with headroom.
const DefaultLiveCallCap = 250

var liveBudget *LiveCallBudget

func resetLiveCallBudget() {
	cap := DefaultLiveCallCap
	if v := strings.TrimSpace(os.Getenv("JUKE_EVALS_LIVE_CALL_CAP")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cap = n
		}
	}
	if cap <= 0 {
		liveBudget = nil
		return
	}
	liveBudget = &LiveCallBudget{cap: cap}
}

func liveCallsEnabled() bool {
	return os.Getenv("JUKE_EVALS_LIVE") == "1"
}

func consumeLiveCall() error {
	if !liveCallsEnabled() || liveBudget == nil {
		return nil
	}
	liveBudget.mu.Lock()
	defer liveBudget.mu.Unlock()
	liveBudget.count++
	if liveBudget.count > liveBudget.cap {
		return fmt.Errorf("live eval call budget exceeded: %d calls (cap %d)", liveBudget.count, liveBudget.cap)
	}
	return nil
}

func liveCallCount() int {
	if liveBudget == nil {
		return 0
	}
	liveBudget.mu.Lock()
	n := liveBudget.count
	liveBudget.mu.Unlock()
	return n
}

// LiveBudgetStatus returns call count/cap when a live budget is active (for reports/logs).
func LiveBudgetStatus() string {
	return liveBudgetReport()
}
