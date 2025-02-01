package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk-starter-kit/programs"
	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/stretchr/testify/require"
)

func TestCanFundFeed(t *testing.T) {
	simpleFeed := &RegisterFeed{
		FeedID:             1,
		FeedName:           "A Soccer Match",
		RewardPerRound:     10,
		RewardVaultInitial: 1000,
		MinDeposit:         100000,
		AppealEffect:       0,
		AppealMaxDelay:     0,
		FinalizeInterval:   5000, // 5000 ms
		ProgramID:          programs.BinaryAggregatorProgramID,
		Memo:               []byte("some random memo"),
	}
	simpleFeedRaw, err := simpleFeed.Marshal()
	require.NoError(t, err)

	fundFeed := &FundFeed{
		FeedID: simpleFeed.FeedID,
		Amount: 1e5,
	}

	testActor := codec.CreateAddress(0, ids.GenerateTestID())
	initialActorBalance := uint64(1e10)
	tests := []chaintest.ActionTest{
		{
			Name:   "SimpleFund",
			Actor:  testActor,
			Action: fundFeed,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				// setup the feed
				err := storage.IncrementFeedID(ctx, store)
				require.NoError(t, err)
				err = storage.SetFeed(ctx, store, simpleFeed.FeedID, simpleFeedRaw)
				require.NoError(t, err)
				err = storage.SetFeedRewardVault(ctx, store, simpleFeed.FeedID, simpleFeed.RewardVaultInitial)
				require.NoError(t, err)

				// setup the actor balance
				err = storage.SetBalance(ctx, store, testActor, initialActorBalance)
				require.NoError(t, err)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				simpleFeedStorageRaw, err := storage.GetFeed(ctx, m, simpleFeed.FeedID)
				require.NoError(t, err)
				require.Equal(t, simpleFeedRaw, simpleFeedStorageRaw)
				vaultValue, err := storage.GetFeedRewardVault(ctx, m, simpleFeed.FeedID)
				require.NoError(t, err)
				require.Equal(t, simpleFeed.RewardVaultInitial+fundFeed.Amount, vaultValue)
				actorBalance, err := storage.GetBalance(ctx, m, testActor)
				require.NoError(t, err)
				require.Equal(t, initialActorBalance-fundFeed.Amount, actorBalance)
			},
			ExpectedOutputs: &FundFeedResult{
				FeedID: fundFeed.FeedID,
				Amount: fundFeed.Amount,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func TestCannotFundFeedNotExists(t *testing.T) {
	fundFeed := &FundFeed{
		FeedID: 0,
		Amount: 1e5,
	}

	testActor := codec.CreateAddress(0, ids.GenerateTestID())
	initialActorBalance := uint64(1e10)
	tests := []chaintest.ActionTest{
		{
			Name:   "FundFeedNotExists",
			Actor:  testActor,
			Action: fundFeed,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				// setup the actor balance
				err := storage.SetBalance(ctx, store, testActor, initialActorBalance)
				require.NoError(t, err)
				return store
			}(),
			ExpectedErr: ErrFundFeedNotExists,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
