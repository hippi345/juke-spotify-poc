package voting

import (
	"log"
	"sync"
	"time"

	"juke-spotify-poc/server/spotify"
)

const (
	tickerInterval = 3 * time.Second
)

// Ticker polls Spotify and advances rounds. Voting runs during the current song until ~15s left;
// at that point we add the winner to the queue (plays after current song ends) and create the next round.
type Ticker struct {
	manager *Manager
	svc     *spotify.Client
	stopCh  chan struct{}
	doneCh  chan struct{}
	mu      sync.Mutex
}

// NewTicker creates a new ticker
func NewTicker(manager *Manager, svc *spotify.Client) *Ticker {
	return &Ticker{
		manager: manager,
		svc:    svc,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start begins the ticker loop (non-blocking)
func (t *Ticker) Start() {
	t.mu.Lock()
	select {
	case <-t.stopCh:
		t.stopCh = make(chan struct{})
		t.doneCh = make(chan struct{})
	default:
		// Already running
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()

	go t.run()
}

// Stop stops the ticker
func (t *Ticker) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	select {
	case <-t.stopCh:
		return
	default:
		close(t.stopCh)
	}
	<-t.doneCh
}

func (t *Ticker) run() {
	defer close(t.doneCh)

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.tick()
		}
	}
}

func (t *Ticker) tick() {
	cp, err := t.svc.GetCurrentlyPlaying()
	if err != nil {
		return
	}

	// Start new round when winner (queued) has begun playing
	t.manager.StartNewRoundIfWinnerPlaying(cp)

	session, round, _ := t.manager.GetState(cp)
	if session == nil || session.Status != "active" {
		return
	}

	// Recovery: session active but no round (e.g. fetch failed) - try to create one
	if round == nil {
		if err := t.manager.RecoverRound(); err != nil {
			log.Printf("voting: recover round: %v", err)
		}
		return
	}

	// Advance when: (1) song has ~15s left (queue winner, let current finish), or (2) nothing playing and round time expired
	shouldAdvance := false
	var remainingMs int64 = -1
	if cp == nil || cp.Item == nil {
		// Nothing playing: advance when round time expired (song finished; don't interrupt on API lag)
		shouldAdvance = time.Now().After(round.RoundEndsAt)
	} else {
		remainingMs = int64(cp.Item.DurationMs) - cp.ProgressMs
		shouldAdvance = remainingMs <= roundEndBufferMs
	}

	if shouldAdvance {
		log.Printf("voting: tick advancing (remainingMs=%d cp=%v)", remainingMs, cp != nil && cp.Item != nil)
		if err := t.manager.AdvanceRound(); err != nil {
			log.Printf("voting: advance round: %v", err)
		}
	}
}
