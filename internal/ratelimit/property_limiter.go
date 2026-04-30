package ratelimit

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// PropertyLimiter enforces independent rate limits per property key.
type PropertyLimiter struct {
	mu       sync.Mutex
	qpm      int
	burst    int
	limiters map[string]*rate.Limiter
}

// NewPropertyLimiter creates a per-property limiter with the provided QPM and burst.
func NewPropertyLimiter(qpm int, burst int) *PropertyLimiter {
	if qpm <= 0 {
		qpm = 1
	}
	if burst <= 0 {
		burst = 1
	}

	return &PropertyLimiter{
		qpm:      qpm,
		burst:    burst,
		limiters: make(map[string]*rate.Limiter),
	}
}

// Wait blocks until a token is available for the given property.
func (p *PropertyLimiter) Wait(ctx context.Context, property string) error {
	return p.getLimiter(property).Wait(ctx)
}

// Size returns the number of initialized property limiters.
func (p *PropertyLimiter) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.limiters)
}

func (p *PropertyLimiter) getLimiter(property string) *rate.Limiter {
	p.mu.Lock()
	defer p.mu.Unlock()

	if limiter, ok := p.limiters[property]; ok {
		return limiter
	}

	perSecond := rate.Limit(float64(p.qpm) / 60.0)
	limiter := rate.NewLimiter(perSecond, p.burst)
	p.limiters[property] = limiter

	return limiter
}
