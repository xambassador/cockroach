// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package ycsb

import (
	"math/rand/v2"

	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/workload/workloadimpl"
)

// SkewedLatestGenerator is a random number generator that generates numbers in
// the range [iMin, iMax], but skews it towards iMax using a zipfian
// distribution.
type SkewedLatestGenerator struct {
	mu struct {
		syncutil.Mutex
		iMax    uint64
		zipfGen *workloadimpl.ZipfGenerator
	}
}

// NewSkewedLatestGenerator constructs a new SkewedLatestGenerator with the
// given parameters. It returns an error if the parameters are outside the
// accepted range.
func NewSkewedLatestGenerator(
	rng *rand.Rand, iMin, iMax uint64, theta float64, verbose bool,
) (*SkewedLatestGenerator, error) {

	z := SkewedLatestGenerator{}
	z.mu.iMax = iMax
	zipfGen, err := workloadimpl.NewZipfGenerator(rng, 0, iMax-iMin, theta, verbose)
	if err != nil {
		return nil, err
	}
	z.mu.zipfGen = zipfGen

	return &z, nil
}

// IncrementIMax increments iMax by count.
func (z *SkewedLatestGenerator) IncrementIMax(count uint64) error {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.mu.iMax += count
	return z.mu.zipfGen.IncrementIMax(count)
}

// Uint64 returns a random Uint64 between iMin and iMax, where keys near iMax
// are most likely to be drawn.
func (z *SkewedLatestGenerator) Uint64() uint64 {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.mu.iMax - z.mu.zipfGen.Uint64()
}
