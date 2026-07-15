package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func makeInputs(values ...int64) []SplitInput {
	out := make([]SplitInput, len(values))
	for i, v := range values {
		out[i] = SplitInput{UserID: uuid.New(), Value: v}
	}
	return out
}

func sum(ss []SplitInput) int64 {
	var s int64
	for _, x := range ss {
		s += x.Value
	}
	return s
}

func TestResolveSplitsEqual(t *testing.T) {
	in := makeInputs(0, 0, 0)
	out, err := resolveSplits(SplitEqual, 1000, in)
	require.NoError(t, err)
	require.Len(t, out, 3)
	// 1000 / 3 = 333 r 1 → shares: 334,333,333
	require.EqualValues(t, 334, out[0].ShareCents)
	require.EqualValues(t, 333, out[1].ShareCents)
	require.EqualValues(t, 333, out[2].ShareCents)
}

func TestResolveSplitsExact(t *testing.T) {
	in := makeInputs(200, 800)
	out, err := resolveSplits(SplitExact, 1000, in)
	require.NoError(t, err)
	require.EqualValues(t, 200, out[0].ShareCents)
	require.EqualValues(t, 800, out[1].ShareCents)

	// Mismatch fails
	_, err = resolveSplits(SplitExact, 999, makeInputs(200, 800))
	require.Error(t, err)
}

func TestResolveSplitsPercent(t *testing.T) {
	in := makeInputs(3333, 3333, 3334) // basis points summing to 10000
	out, err := resolveSplits(SplitPercent, 1000, in)
	require.NoError(t, err)
	var total int64
	for _, s := range out {
		total += s.ShareCents
	}
	require.EqualValues(t, 1000, total)

	_, err = resolveSplits(SplitPercent, 1000, makeInputs(5000, 5001))
	require.Error(t, err)
}

func TestResolveSplitsShares(t *testing.T) {
	in := makeInputs(1, 2, 3)
	out, err := resolveSplits(SplitShares, 600, in)
	require.NoError(t, err)
	require.EqualValues(t, 100, out[0].ShareCents)
	require.EqualValues(t, 200, out[1].ShareCents)
	require.EqualValues(t, 300, out[2].ShareCents)

	// With rounding: 100 cents, 3 shares -> 33,33,33 + rem 1 on first
	in2 := makeInputs(1, 1, 1)
	out2, err := resolveSplits(SplitShares, 100, in2)
	require.NoError(t, err)
	var total int64
	for _, s := range out2 {
		total += s.ShareCents
	}
	require.EqualValues(t, 100, total)

	_ = sum // silence unused
}

func TestResolveSplitsDuplicateUser(t *testing.T) {
	id := uuid.New()
	_, err := resolveSplits(SplitEqual, 100, []SplitInput{{UserID: id}, {UserID: id}})
	require.Error(t, err)
}

func TestDefaultOccurredAtAnchorsNoonUTC(t *testing.T) {
	got := defaultOccurredAt()
	now := time.Now().UTC()
	require.Equal(t, now.Year(), got.Year())
	require.Equal(t, now.Month(), got.Month())
	require.Equal(t, now.Day(), got.Day())
	require.Equal(t, 12, got.Hour())
	require.Equal(t, 0, got.Minute())
	require.Equal(t, 0, got.Second())
	require.Equal(t, time.UTC, got.Location())
}
