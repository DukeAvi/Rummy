// Package lan provides an authoritative, two-seat local-network session.
package lan

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/DukeAvi/rummy-tui/internal/game"
	"sync"
	"time"
)

const Version = 1
const ResumeTTL = 5 * time.Minute
const HeartbeatTimeout = 5 * time.Second

type Request struct {
	Version       int
	Token, Action string
	Seq, Round    uint64
	IDs           []uint64
	ID            uint64
	Open          bool
	Config        game.Config
}
type Response struct {
	Error      string
	Token      string `json:",omitempty"`
	Player     int
	Round, Seq uint64
	Config     game.Config
	Ready      [2]bool
	Connected  bool
	Closed     bool
	View       *game.PrivateView `json:",omitempty"`
}
type Session struct {
	created            time.Time
	lastError          [2]string
	mu                 sync.Mutex
	host, join, resume string
	joined, closed     bool
	lastSeen           time.Time
	ready              [2]bool
	seq                [2]uint64
	round              uint64
	cfg                game.Config
	g                  *game.Game
}

func secret() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func New(cfg game.Config) (*Session, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Session{host: secret(), join: secret(), cfg: cfg, created: time.Now()}, nil
}
func (s *Session) HostToken() string { return s.host }
func (s *Session) JoinToken() string { return s.join }
func (s *Session) Close()            { s.mu.Lock(); defer s.mu.Unlock(); s.closed = true }

// Handle serializes all game mutations; a token determines the seat, never a
// caller-supplied player index. Action sequence numbers make retries idempotent.
func (s *Session) Handle(r Request) Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Response{Player: -1}
	if r.Version != Version {
		out.Error = "incompatible protocol version; install the same game version"
		return out
	}
	if s.closed {
		out.Error = "session ended"
		out.Closed = true
		return out
	}
	if r.Action == "join" && !s.joined && r.Token == s.join && time.Since(s.created) >= 10*time.Minute {
		out.Error = "join code expired; ask the host to create a fresh lobby"
		return out
	}
	p := -1
	switch {
	case r.Token == s.host:
		p = 0
	case r.Action == "join" && !s.joined && r.Token == s.join && time.Since(s.created) < 10*time.Minute:
		s.joined = true
		s.resume = secret()
		s.lastSeen = time.Now()
		p = 1
		out.Token = s.resume
	case s.joined && r.Token == s.resume:
		if time.Since(s.lastSeen) > ResumeTTL {
			out.Error = "rejoin code expired; ask the host for a new game"
			return out
		}
		p = 1
		s.lastSeen = time.Now()
	default:
		out.Error = "incorrect or used join code; this game may already be full"
		return out
	}
	out.Player = p
	connected := s.joined && time.Since(s.lastSeen) < HeartbeatTimeout
	var err error
	if r.Action != "poll" && r.Action != "join" {
		if r.Seq == s.seq[p] && r.Seq != 0 { // Retry of a request whose reply was lost.
			if s.lastError[p] != "" {
				err = fmt.Errorf("%s", s.lastError[p])
			}
		} else if r.Seq != s.seq[p]+1 {
			err = fmt.Errorf("stale action sequence; reconnect before continuing")
		} else {
			s.seq[p] = r.Seq
			switch r.Action {
			case "leave":
				if p == 0 {
					s.closed = true
				} else {
					s.joined = false
					s.resume = ""
					s.closed = true
				}
			case "configure":
				if p != 0 {
					err = fmt.Errorf("only the host can choose the next round")
				} else if s.g != nil && s.g.Phase() != game.Finished {
					err = fmt.Errorf("finish the current round first")
				} else if err = r.Config.Validate(); err == nil {
					s.cfg = r.Config
					s.ready = [2]bool{}
					s.g = nil
					s.round++
				}
			case "ready":
				if s.g != nil {
					err = fmt.Errorf("round already started")
				} else if r.Round != s.round {
					err = fmt.Errorf("round setup changed; review it before readying")
				} else {
					s.ready[p] = true
					if s.ready[0] && s.ready[1] && connected {
						s.g, err = game.New(s.cfg, nil)
						if err == nil {
							err = s.g.ConfirmHandoff(0)
						}
					}
				}
			default:
				if !connected {
					err = fmt.Errorf("opponent disconnected; waiting for reconnection")
				} else if s.g == nil || r.Round != s.round {
					err = fmt.Errorf("round changed; refresh before acting")
				} else {
					switch r.Action {
					case "draw":
						err = s.g.DrawCard(p, r.Open)
					case "discard":
						err = s.g.DiscardCard(p, r.ID)
					case "declare":
						err = s.g.Declare(p, r.ID)
					case "group":
						err = s.g.AddGroup(p, r.IDs)
					case "ungroup":
						err = s.g.Ungroup(p, r.ID)
					case "clear":
						err = s.g.ClearGroups(p)
					case "order":
						err = s.g.Reorder(p, r.IDs)
					default:
						err = fmt.Errorf("unknown action")
					}
					if err == nil && s.g.Phase() == game.Handoff {
						err = s.g.ConfirmHandoff(s.g.Active())
					}
				}
			}
		}
	}
	if s.g == nil && s.ready[0] && s.ready[1] && connected && !s.closed {
		s.g, err = game.New(s.cfg, nil)
		if err == nil {
			err = s.g.ConfirmHandoff(0)
		}
	}
	if err != nil {
		out.Error = err.Error()
	}
	if r.Action != "poll" && r.Action != "join" && r.Seq == s.seq[p] {
		s.lastError[p] = out.Error
	}
	out.Round = s.round
	out.Seq = s.seq[p]
	out.Ready = s.ready
	out.Config = s.cfg
	out.Connected = connected
	out.Closed = s.closed
	if s.g != nil {
		v := s.g.ViewFor(p)
		out.View = &v
	}
	return out
}
