package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/julian-alarcon/dothesplit/api/internal/repo"
	"github.com/stretchr/testify/require"
)

func TestSimplifyTwoWay(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	debts := Simplify([]repo.NetBalance{
		{UserID: a, NetCents: 100},
		{UserID: b, NetCents: -100},
	})
	require.Len(t, debts, 1)
	require.Equal(t, b, debts[0].FromUserID)
	require.Equal(t, a, debts[0].ToUserID)
	require.EqualValues(t, 100, debts[0].AmountCents)
}

func TestSimplifyChain(t *testing.T) {
	// A is owed 100, B is owed 50, C owes 150.
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()
	debts := Simplify([]repo.NetBalance{
		{UserID: a, NetCents: 100},
		{UserID: b, NetCents: 50},
		{UserID: c, NetCents: -150},
	})
	// Expect two transfers from C: one to A for 100 and one to B for 50.
	require.Len(t, debts, 2)
	var total int64
	for _, d := range debts {
		require.Equal(t, c, d.FromUserID)
		total += d.AmountCents
	}
	require.EqualValues(t, 150, total)
}

func TestSimplifyAllSettled(t *testing.T) {
	require.Empty(t, Simplify([]repo.NetBalance{
		{UserID: uuid.New(), NetCents: 0},
		{UserID: uuid.New(), NetCents: 0},
	}))
}
