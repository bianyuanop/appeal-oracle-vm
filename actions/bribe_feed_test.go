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

func TestInsertBribeFeed(t *testing.T) {
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

	now := time.Now().UnixMilli()
	roundInfo := &common.RoundInfo{
		RoundNumber: 1,
		Start:       now,
		End:         now + simpleFeed.FinalizeInterval,
		Delay:       0,
		Appeals:     nil,
	}

	bribeProvider := codec.CreateAddress(0, ids.GenerateTestID())
	desireValue := []byte("testvalue")
	reporter := codec.CreateAddress(0, ids.GenerateTestID())

	bribeAction := &BribeFeed{
		OpCode:    BribeInsert,
		FeedID:    simpleFeed.FeedID,
		Round:     roundInfo.RoundNumber,
		Recipient: reporter,
		Info: &common.BribeInfo{
			Amount:   simpleFeed.MinDeposit + simpleFeed.RewardPerRound,
			Value:    desireValue,
			Provider: bribeProvider,
		},
	}

	tests := []chaintest.ActionTest{
		{
			Name:   "CanInsertBribe",
			Actor:  bribeProvider,
			Action: bribeAction,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				storage.SetBalance(ctx, store, codec.EmptyAddress, 1e10)
				err := storage.SetFeed(ctx, store, simpleFeed.FeedID, simpleFeedRaw)
				require.NoError(t, err)
				err = storage.SetHighestFeedID(ctx, store, simpleFeed.FeedID)
				require.NoError(t, err)
				err = storage.SetFeedRound(ctx, store, simpleFeed.FeedID, roundInfo)
				require.NoError(t, err)
				err = storage.SetBalance(ctx, store, bribeProvider, bribeAction.Info.Amount)
				require.NoError(t, err)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				bribes, err := storage.GetFeedBribes(ctx, m, simpleFeed.FeedID, reporter, bribeAction.Round)
				require.NoError(t, err)
				require.Equal(t, 1, len(bribes))
				bribe := bribes[0]
				require.Equal(t, bribeAction.Info, bribe)
			},
			ExpectedOutputs: &BribeFeedResult{
				Opcode:    bribeAction.OpCode,
				Round:     bribeAction.Round,
				Amount:    bribeAction.Info.Amount,
				Recipient: reporter,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func TestRemoveBribeFeed(t *testing.T) {
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

	now := time.Now().UnixMilli()
	roundInfo := &common.RoundInfo{
		RoundNumber: 1,
		Start:       now,
		End:         now + simpleFeed.FinalizeInterval,
		Delay:       0,
		Appeals:     nil,
	}

	bribeProvider := codec.CreateAddress(0, ids.GenerateTestID())
	desireValue := []byte("testvalue")
	reporter := codec.CreateAddress(0, ids.GenerateTestID())

	bribeAction := &BribeFeed{
		OpCode:    BribeDelete,
		FeedID:    simpleFeed.FeedID,
		Round:     roundInfo.RoundNumber,
		Recipient: reporter,
		Info: &common.BribeInfo{
			Amount:   simpleFeed.MinDeposit + simpleFeed.RewardPerRound,
			Value:    desireValue,
			Provider: bribeProvider,
		},
	}

	tests := []chaintest.ActionTest{
		{
			Name:   "CanRemoveExistingBribe",
			Actor:  bribeProvider,
			Action: bribeAction,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				storage.SetBalance(ctx, store, codec.EmptyAddress, 1e10)
				err := storage.SetFeed(ctx, store, simpleFeed.FeedID, simpleFeedRaw)
				require.NoError(t, err)
				err = storage.SetHighestFeedID(ctx, store, simpleFeed.FeedID)
				require.NoError(t, err)
				err = storage.SetFeedRound(ctx, store, simpleFeed.FeedID, roundInfo)
				require.NoError(t, err)

				err = storage.SetBalance(ctx, store, bribeProvider, 0)
				require.NoError(t, err)
				err = storage.AddFeedBribe(ctx, store, simpleFeed.FeedID, reporter, bribeAction.Round, bribeAction.Info)
				require.NoError(t, err)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				bribes, err := storage.GetFeedBribes(ctx, m, simpleFeed.FeedID, reporter, bribeAction.Round)
				require.NoError(t, err)
				require.Equal(t, 0, len(bribes))
				bal, err := storage.GetBalance(ctx, m, bribeProvider)
				require.NoError(t, err)
				require.Equal(t, bribeAction.Info.Amount, bal)
			},
			ExpectedOutputs: &BribeFeedResult{
				Opcode:    bribeAction.OpCode,
				Round:     bribeAction.Round,
				Amount:    bribeAction.Info.Amount,
				Recipient: reporter,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
