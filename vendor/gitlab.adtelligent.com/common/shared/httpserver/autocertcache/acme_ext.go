package autocertcache

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"strconv"
	"time"
)

// next function just copied from acme package
func acmeDefaultBackoff(n int, r *http.Request, res *http.Response) time.Duration {
	const max = 10 * time.Second
	var jitter time.Duration
	if x, err := rand.Int(rand.Reader, big.NewInt(1000)); err == nil {
		// Set the minimum to 1ms to avoid a case where
		// an invalid Retry-After value is parsed into 0 below,
		// resulting in the 0 returned value which would unintentionally
		// stop the retries.
		jitter = (1 + time.Duration(x.Int64())) * time.Millisecond
	}
	if v, ok := res.Header["Retry-After"]; ok {
		return retryAfter(v[0]) + jitter
	}

	if n < 1 {
		n = 1
	}
	if n > 30 {
		n = 30
	}
	d := time.Duration(1<<uint(n-1))*time.Second + jitter
	if d > max {
		return max
	}
	return d
}

//next function just copied from acme package

// retryAfter parses a Retry-After HTTP header value,
// trying to convert v into an int (seconds) or use http.ParseTime otherwise.
// It returns zero value if v cannot be parsed.
func retryAfter(v string) time.Duration {
	if i, err := strconv.Atoi(v); err == nil {
		return time.Duration(i) * time.Second
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return 0
	}
	return t.Sub(time.Now())
}
