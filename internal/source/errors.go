package source

import "errors"

// ErrCandidateUnavailable marks missing or invalid title/chapter data. Other
// provider errors abort discovery rather than being hidden as rejections.
var ErrCandidateUnavailable = errors.New("candidate unavailable or invalid")
