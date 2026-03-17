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

// Ticker polls Spotify and advances rounds when the current song has ~15s left
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
	session, round, _ := t.manager.GetState(nil)
	if session == nil || session.Status != "active" || round == nil {
		return
	}

	cp, err := t.svc.GetCurrentlyPlaying()
	if err != nil {
		return
	}
	if cp == nil || cp.Item == nil {
		return
	}

	remainingMs := int64(cp.Item.DurationMs) - cp.ProgressMs
	if remainingMs <= roundEndBufferMs {
		if err := t.manager.AdvanceRound(); err != nil {
			log.Printf("voting: advance round: %v", err)
		}
	}
}
