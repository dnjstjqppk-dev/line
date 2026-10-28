package server

import (
	"net"
	"sync"
	"time"
)

type authWindow struct {
	start time.Time
	count int
}
type authLimiter struct {
	mu      sync.Mutex
	windows map[string]authWindow
	cleanup time.Time
}

func (l *authLimiter) allow(remote, route string, now time.Time) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	key := host + "|" + route
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windows == nil {
		l.windows = make(map[string]authWindow)
	}
	if now.Sub(l.cleanup) >= time.Minute {
		for k, v := range l.windows {
			if now.Sub(v.start) >= time.Minute {
				delete(l.windows, k)
			}
		}
		l.cleanup = now
	}
	v, ok := l.windows[key]
	if !ok && len(l.windows) >= 10000 {
		return false
	}
	if now.Sub(v.start) >= time.Minute {
		v = authWindow{start: now}
	}
	if v.count >= 60 {
		return false
	}
	v.count++
	l.windows[key] = v
	return true
}
