package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/bianyuanop/oraclevm/programs"
	"github.com/bianyuanop/oraclevm/storage"
	"github.com/stretchr/testify/require"
)

func TestDepositFeed(t *testing.T) {
	simpleFeed := &RegisterFeed{
		FeedID:           1,
		FeedName:         "A Soccer Match",
		MinDeposit:       100000,
		AppealEffect:     0,
		AppealMaxDelay:   0,
		FinalizeInterval: 5000, // 5000 ms
		ProgramID:        programs.BinaryAggregatorProgramID,
		Memo:             []byte("some random memo"),
	}
	simpleFeedRaw, err := simpleFeed.Marshal()
	require.NoError(t, err)

	depositAction := &DepositFeed{
		FeedID: simpleFeed.FeedID,
		Amount: 1e9,
	}

	tests := []chaintest.ActionTest{
		{
			Name:   "CanDepositIntoExisting",
			Actor:  codec.EmptyAddress,
			Action: depositAction,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				storage.SetBalance(ctx, store, codec.EmptyAddress, 1e10)
				err := storage.SetFeed(ctx, store, simpleFeed.FeedID, simpleFeedRaw)
				require.NoError(t, err)
				err = storage.SetHighestFeedID(ctx, store, simpleFeed.FeedID)
				require.NoError(t, err)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				deposit, err := storage.GetFeedDeposit(ctx, m, simpleFeed.FeedID, codec.EmptyAddress)
				require.NoError(t, err)
				require.Equal(t, depositAction.Amount, deposit)
			},
			ExpectedOutputs: &DepositFeedResult{
				FeedID:  simpleFeed.FeedID,
				Amount:  1e9,
				Address: codec.EmptyAddress,
			},
		},
		{
			Name:   "CannotDepositWithEnoughBalance",
			Actor:  codec.EmptyAddress,
			Action: depositAction,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				storage.SetBalance(ctx, store, codec.EmptyAddress, 1e8)
				err := storage.SetFeed(ctx, store, simpleFeed.FeedID, simpleFeedRaw)
				require.NoError(t, err)
				err = storage.SetHighestFeedID(ctx, store, simpleFeed.FeedID)
				require.NoError(t, err)
				return store
			}(),
			ExpectedErr: ErrBalanceBelowDepositAmount,
		},
		{
			Name:   "CannotDepositIntoNonExisting",
			Actor:  codec.EmptyAddress,
			Action: depositAction,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.SetBalance(ctx, store, codec.EmptyAddress, 1e10)
				require.NoError(t, err)
				return store
			}(),
			ExpectedErr: ErrDepositFeedGreaterThanHighest,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
