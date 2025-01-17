package kzg

import (
	"fmt"
	"math/rand"
	"testing"

	bls "github.com/Layr-Labs/eigenda/pkg/kzg/bn254"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFFTSettings_RecoverPolyFromSamples_Simple(t *testing.T) {
	// Create some random data, with padding...
	fs := NewFFTSettings(2)
	poly := make([]bls.Fr, fs.MaxWidth)
	for i := uint64(0); i < fs.MaxWidth/2; i++ {
		bls.AsFr(&poly[i], i)
	}
	for i := fs.MaxWidth / 2; i < fs.MaxWidth; i++ {
		poly[i] = bls.ZERO
	}

	// Get data for polynomial SLOW_INDICES
	data, err := fs.FFT(poly, false)
	require.Nil(t, err)

	subset := make([]*bls.Fr, fs.MaxWidth)
	subset[0] = &data[0]
	subset[3] = &data[3]

	recovered, err := fs.RecoverPolyFromSamples(subset, fs.ZeroPolyViaMultiplication)
	require.Nil(t, err)

	for i := range recovered {
		assert.True(t, bls.EqualFr(&recovered[i], &data[i]),
			"recovery at index %d got %s but expected %s", i, bls.FrStr(&recovered[i]), bls.FrStr(&data[i]))
	}

	// And recover the original coeffs for good measure
	back, err := fs.FFT(recovered, true)
	require.Nil(t, err)

	for i := uint64(0); i < fs.MaxWidth/2; i++ {
		assert.True(t, bls.EqualFr(&back[i], &poly[i]),
			"coeff at index %d got %s but expected %s", i, bls.FrStr(&back[i]), bls.FrStr(&poly[i]))
	}

	for i := fs.MaxWidth / 2; i < fs.MaxWidth; i++ {
		assert.True(t, bls.EqualZero(&back[i]),
			"expected zero padding in index %d", i)
	}
}

func TestFFTSettings_RecoverPolyFromSamples(t *testing.T) {
	// Create some random poly, with padding so we get redundant data
	fs := NewFFTSettings(10)
	poly := make([]bls.Fr, fs.MaxWidth)
	for i := uint64(0); i < fs.MaxWidth/2; i++ {
		bls.AsFr(&poly[i], i)
	}
	for i := fs.MaxWidth / 2; i < fs.MaxWidth; i++ {
		poly[i] = bls.ZERO
	}

	// Get coefficients for polynomial SLOW_INDICES
	data, err := fs.FFT(poly, false)
	require.Nil(t, err)

	// Util to pick a random subnet of the values
	randomSubset := func(known uint64, rngSeed uint64) []*bls.Fr {
		withMissingValues := make([]*bls.Fr, fs.MaxWidth)
		for i := range data {
			withMissingValues[i] = &data[i]
		}
		rng := rand.New(rand.NewSource(int64(rngSeed)))
		missing := fs.MaxWidth - known
		pruned := rng.Perm(int(fs.MaxWidth))[:missing]
		for _, i := range pruned {
			withMissingValues[i] = nil
		}
		return withMissingValues
	}

	// Try different amounts of known indices, and try it in multiple random ways
	var lastKnown uint64 = 0
	for knownRatio := 0.7; knownRatio < 1.0; knownRatio += 0.05 {
		known := uint64(float64(fs.MaxWidth) * knownRatio)
		if known == lastKnown {
			continue
		}
		lastKnown = known
		for i := 0; i < 3; i++ {
			t.Run(fmt.Sprintf("random_subset_%d_known_%d", i, known), func(t *testing.T) {
				subset := randomSubset(known, uint64(i))

				recovered, err := fs.RecoverPolyFromSamples(subset, fs.ZeroPolyViaMultiplication)
				require.Nil(t, err)

				for i := range recovered {
					assert.True(t, bls.EqualFr(&recovered[i], &data[i]),
						"recovery at index %d got %s but expected %s", i, bls.FrStr(&recovered[i]), bls.FrStr(&data[i]))
				}

				// And recover the original coeffs for good measure
				back, err := fs.FFT(recovered, true)
				require.Nil(t, err)

				half := uint64(len(back)) / 2
				for i := uint64(0); i < half; i++ {
					assert.True(t, bls.EqualFr(&back[i], &poly[i]),
						"coeff at index %d got %s but expected %s", i, bls.FrStr(&back[i]), bls.FrStr(&poly[i]))
				}
				for i := half; i < fs.MaxWidth; i++ {
					assert.True(t, bls.EqualZero(&back[i]),
						"expected zero padding in index %d", i)
				}
			})
		}
	}
}
