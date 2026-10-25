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

// liveCallCapHeadroom is extra calls above the per-case formula (generator + searches + judge runs).
const liveCallCapHeadroom = 50

// liveCallCapFloor is the minimum cap when the computed budget is small (e.g. short CI subsets).
const liveCallCapFloor = 400

// DefaultLiveCallCap is derived from the full eval matrix at init (see computeLiveCallCap).
var DefaultLiveCallCap = computeLiveCallCap(FilterCases(false), judgeRunsPerCase)

// computeLiveCallCap estimates outbound Gemini/Spotify calls for one live suite arm:
// per case: 1 generator + TrackCount searches (unless resolve N/A) + judgeRuns judge calls.
func computeLiveCallCap(cases []Case, judgeRuns int) int {
	if judgeRuns <= 0 {
		judgeRuns = judgeRunsPerCase
	}
	total := 0
	for _, c := range cases {
		total++ // generator
		if c.ExpectError {
			continue
		}
		if !c.ForceResolveNA {
			total += c.TrackCount
		}
		total += judgeRuns
	}
	total += liveCallCapHeadroom
	if total < liveCallCapFloor {
		return liveCallCapFloor
	}
	return total
}

var liveBudget *LiveCallBudget

func resetLiveCallBudget() {
	cap := computeLiveCallCap(FilterCases(false), judgeRunsForSuite())
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
