package services

import (
	"fmt"
	"hash/fnv"
	"math/rand"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
	"github.com/021labs-ai/synthetic-worlds/pkg/apierror"
)

// SyntheticError represents a simulated error from error injection.
type SyntheticError struct {
	Code       domain.SyntheticErrorCode
	StatusCode int
	Message    string
}

func (e *SyntheticError) Error() string {
	return e.Message
}

// maybeInjectError deterministically injects an error based on the failure profile.
// Uses a seeded RNG with (world_seed, step_count) so the same world replayed
// from step 0 produces identical failures every time.
func maybeInjectError(fp *domain.FailureProfile, seed *int, stepCount int) *apierror.APIError {
	if fp == nil || fp.Rate <= 0 || len(fp.Codes) == 0 {
		return nil
	}

	seedVal := 0
	if seed != nil {
		seedVal = *seed
	}

	h := fnv.New64a()
	fmt.Fprintf(h, "%d:%d", seedVal, stepCount)
	rng := rand.New(rand.NewSource(int64(h.Sum64())))

	if rng.Float64() >= fp.Rate {
		return nil
	}

	code := fp.Codes[rng.Intn(len(fp.Codes))]
	status, ok := domain.ErrorCodeToHTTPStatus[code]
	if !ok {
		status = 500
	}

	return apierror.New(
		string(code),
		fmt.Sprintf("Simulated error: %s", code),
		status,
	)
}
