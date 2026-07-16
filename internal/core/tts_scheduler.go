package core

import (
	"context"
	"errors"
	"sync"
)

var errTTSSuperseded = errors.New("TTS request superseded by a newer playback session")

type ttsScheduleMeta struct {
	Owner     string
	SessionID int64
	RequestID int64
}

type ttsScheduleTicket struct {
	meta    ttsScheduleMeta
	ctx     context.Context
	cancel  context.CancelCauseFunc
	ready   chan struct{}
	granted bool
	err     error
}

// ttsRequestScheduler keeps the CPU-heavy ONNX engine single-flight while
// allowing a user's newer playback session to invalidate that user's stale
// active and queued work. Chunks within one session retain FIFO ordering.
type ttsRequestScheduler struct {
	mu            sync.Mutex
	active        *ttsScheduleTicket
	pending       []*ttsScheduleTicket
	latestSession map[string]int64
}

func newTTSRequestScheduler() *ttsRequestScheduler {
	return &ttsRequestScheduler{latestSession: make(map[string]int64)}
}

func (s *ttsRequestScheduler) acquire(ctx context.Context, meta ttsScheduleMeta) (context.Context, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancelCause(ctx)
	ticket := &ttsScheduleTicket{
		meta:   meta,
		ctx:    runCtx,
		cancel: cancel,
		ready:  make(chan struct{}),
	}

	s.mu.Lock()
	if meta.Owner != "" {
		latest, exists := s.latestSession[meta.Owner]
		if exists && meta.SessionID < latest {
			s.mu.Unlock()
			cancel(errTTSSuperseded)
			return nil, nil, errTTSSuperseded
		}
		if !exists || meta.SessionID > latest {
			s.latestSession[meta.Owner] = meta.SessionID
			s.supersedeOlderLocked(meta.Owner, meta.SessionID)
		}
	}
	s.pending = append(s.pending, ticket)
	s.grantNextLocked()
	s.mu.Unlock()

	select {
	case <-ticket.ready:
		s.mu.Lock()
		err := ticket.err
		granted := ticket.granted
		s.mu.Unlock()
		if err != nil || !granted {
			if err == nil {
				err = context.Canceled
			}
			return nil, nil, err
		}
		var once sync.Once
		release := func() {
			once.Do(func() { s.release(ticket) })
		}
		return runCtx, release, nil
	case <-ctx.Done():
		s.cancelTicket(ticket, ctx.Err())
		return nil, nil, ctx.Err()
	}
}

func (s *ttsRequestScheduler) supersedeOlderLocked(owner string, sessionID int64) {
	if s.active != nil && s.active.meta.Owner == owner && s.active.meta.SessionID < sessionID {
		s.active.err = errTTSSuperseded
		s.active.cancel(errTTSSuperseded)
	}
	for _, ticket := range s.pending {
		if ticket.meta.Owner == owner && ticket.meta.SessionID < sessionID && !ticket.granted && ticket.err == nil {
			ticket.err = errTTSSuperseded
			ticket.cancel(errTTSSuperseded)
			close(ticket.ready)
		}
	}
}

func (s *ttsRequestScheduler) grantNextLocked() {
	if s.active != nil {
		return
	}
	for len(s.pending) > 0 {
		ticket := s.pending[0]
		s.pending = s.pending[1:]
		if ticket.err != nil {
			continue
		}
		ticket.granted = true
		s.active = ticket
		close(ticket.ready)
		return
	}
}

func (s *ttsRequestScheduler) cancelTicket(ticket *ttsScheduleTicket, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ticket.err == nil {
		ticket.err = err
		ticket.cancel(err)
	}
	if s.active == ticket {
		// acquire selected ctx.Done before returning a release function, so the
		// scheduler itself must relinquish a concurrently granted ticket.
		s.active = nil
		s.grantNextLocked()
		return
	}
	if !ticket.granted {
		// The waiter selected ctx.Done; no ready notification is necessary.
		s.grantNextLocked()
	}
}

func (s *ttsRequestScheduler) release(ticket *ttsScheduleTicket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == ticket {
		s.active = nil
		ticket.cancel(context.Canceled)
	}
	s.grantNextLocked()
}
