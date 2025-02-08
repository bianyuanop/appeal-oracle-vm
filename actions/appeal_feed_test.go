package actions

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/bianyuanop/oraclevm/common"
	"github.com/bianyuanop/oraclevm/programs"
	"github.com/bianyuanop/oraclevm/storage"
	"github.com/stretchr/testify/require"
)

func TestAppealCanDelay(t *testing.T) {
	simpleFeed := &RegisterFeed{
		FeedID:           1,
		FeedName:         "A Soccer Match",
		MinDeposit:       100000,
		AppealFee:        10000000,
		AppealEffect:     1000,  // 1s
		AppealMaxDelay:   10000, // 10s
		FinalizeInterval: 5000,  // 5000 ms
		ProgramID:        programs.BinaryAggregatorProgramID,
		Memo:             []byte("some random memo"),
	}
	simpleFeedRaw, err := simpleFeed.Marshal()
	require.NoError(t, err)

	initialRoundInfo := &common.RoundInfo{
		RoundNumber: 1,
		Start:       time.Now().UnixMilli(),
		End:         time.Now().UnixMilli() + simpleFeed.FinalizeInterval,
	}

	testActor := codec.CreateAddress(0, ids.GenerateTestID())
	tests := []chaintest.ActionTest{
		{
			Name:  "SimpleAppeal",
			Actor: testActor,
			Action: &AppealFeed{
				FeedID: simpleFeed.FeedID,
			},
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.IncrementFeedID(ctx, store)
				require.NoError(t, err)
				err = storage.SetFeed(ctx, store, simpleFeed.FeedID, simpleFeedRaw)
				require.NoError(t, err)
				err = storage.SetFeedRound(ctx, store, simpleFeed.FeedID, initialRoundInfo)
				require.NoError(t, err)

				// setup actor balance & vault
				err = storage.SetBalance(ctx, store, testActor, simpleFeed.AppealFee)
				require.NoError(t, err)
				err = storage.SetFeedRewardVault(ctx, store, simpleFeed.FeedID, simpleFeed.RewardVaultInitial)
				require.NoError(t, err)

				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				roundInfo, err := storage.GetFeedRound(ctx, m, simpleFeed.FeedID)
				require.NoError(t, err)
				require.Equal(t, 1, len(roundInfo.Appeals))
				require.Equal(t, simpleFeed.AppealEffect, roundInfo.Delay)

				vaultValue, err := storage.GetFeedRewardVault(ctx, m, simpleFeed.FeedID)
				require.NoError(t, err)
				require.Equal(t, simpleFeed.RewardVaultInitial+simpleFeed.AppealFee, vaultValue)
			},
			ExpectedOutputs: &AppealFeedResult{
				FeedID:  simpleFeed.FeedID,
				EndTime: initialRoundInfo.End + simpleFeed.AppealEffect,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func TestAppealMaxDelay(t *testing.T) {
	simpleFeed := &RegisterFeed{
		FeedID:           1,
		FeedName:         "A Soccer Match",
		MinDeposit:       100000,
		AppealFee:        10000000,
		AppealEffect:     10000, // 10s
		AppealMaxDelay:   1000,  // 1
		FinalizeInterval: 5000,  // 5000 ms
		ProgramID:        programs.BinaryAggregatorProgramID,
		Memo:             []byte("some random memo"),
	}
	simpleFeedRaw, err := simpleFeed.Marshal()
	require.NoError(t, err)

	initialRoundInfo := &common.RoundInfo{
		RoundNumber: 1,
		Start:       time.Now().UnixMilli(),
		End:         time.Now().UnixMilli() + simpleFeed.FinalizeInterval,
	}

	testActor := codec.CreateAddress(0, ids.GenerateTestID())
	tests := []chaintest.ActionTest{
		{
			Name:  "SimpleAppeal",
			Actor: testActor,
			Action: &AppealFeed{
				FeedID: simpleFeed.FeedID,
			},
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.IncrementFeedID(ctx, store)
				require.NoError(t, err)
				err = storage.SetFeed(ctx, store, simpleFeed.FeedID, simpleFeedRaw)
				require.NoError(t, err)
				err = storage.SetFeedRound(ctx, store, simpleFeed.FeedID, initialRoundInfo)
				require.NoError(t, err)

				// setup actor balance & vault
				err = storage.SetBalance(ctx, store, testActor, simpleFeed.AppealFee)
				require.NoError(t, err)
				err = storage.SetFeedRewardVault(ctx, store, simpleFeed.FeedID, simpleFeed.RewardVaultInitial)
				require.NoError(t, err)

				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				roundInfo, err := storage.GetFeedRound(ctx, m, simpleFeed.FeedID)
				require.NoError(t, err)
				require.Equal(t, 1, len(roundInfo.Appeals))
				require.Equal(t, simpleFeed.AppealMaxDelay, roundInfo.Delay)

				vaultValue, err := storage.GetFeedRewardVault(ctx, m, simpleFeed.FeedID)
				require.NoError(t, err)
				require.Equal(t, simpleFeed.RewardVaultInitial+simpleFeed.AppealFee, vaultValue)
			},
			ExpectedOutputs: &AppealFeedResult{
				FeedID:  simpleFeed.FeedID,
				EndTime: initialRoundInfo.End + simpleFeed.AppealMaxDelay,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
