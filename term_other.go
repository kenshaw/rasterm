//go:build !unix

package rasterm

import (
	"context"
	"os"
	"time"
)

// canQueryTerm returns false. The package does not query the terminal on
// platforms other than Unix, so [Detect] reads the environment.
func canQueryTerm(in, out *os.File, getenv func(string) string) bool {
	return false
}

// termQuery returns [ErrNonTTY]. [canQueryTerm] is false, so it is never
// called.
func termQuery(ctx context.Context, in, out *os.File, req string, timeout time.Duration) ([][]byte, []byte, error) {
	return nil, nil, ErrNonTTY
}
