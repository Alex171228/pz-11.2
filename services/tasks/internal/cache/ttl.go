package cache

import (
	"math/rand"
	"time"
)

func TTLWithJitter(base, jitter time.Duration) time.Duration {
	if jitter <= 0 {
		return base
	}
	extra := time.Duration(rand.Int63n(int64(jitter) + 1))
	return base + extra
}
