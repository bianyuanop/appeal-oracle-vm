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

func TestRegisterFeed(t *testing.T) {
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

	testActor := codec.CreateAddress(0, ids.GenerateTestID())
	tests := []chaintest.ActionTest{
		{
			Name:   "SimpleRegistration",
			Actor:  testActor,
			Action: simpleFeed,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.SetBalance(ctx, store, testActor, 1e10)
				require.NoError(t, err)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				simpleFeedStorageRaw, err := storage.GetFeed(ctx, m, uint64(1))
				require.NoError(t, err)
				require.Equal(t, simpleFeedRaw, simpleFeedStorageRaw)
			},
			ExpectedOutputs: &RegisterFeedResult{
				FeedID: 1,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func TestCantRegisterInconsectiveFeed(t *testing.T) {
	simpleFeed := &RegisterFeed{
		FeedID:           2, // the default first one is 1
		FeedName:         "A Soccer Match",
		MinDeposit:       100000,
		AppealEffect:     0,
		AppealMaxDelay:   0,
		FinalizeInterval: 5000, // 5000 ms
		ProgramID:        programs.BinaryAggregatorProgramID,
		Memo:             []byte("some random memo"),
	}
	testActor := codec.CreateAddress(0, ids.GenerateTestID())

	tests := []chaintest.ActionTest{
		{
			Name:   "SimpleRegistration",
			Actor:  testActor,
			Action: simpleFeed,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.SetBalance(ctx, store, testActor, 1e10)
				require.NoError(t, err)
				return store
			}(),
			ExpectedErr: ErrRequestedFeedIDNotLatest,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func TestCanRegistrationWithInitialRewards(t *testing.T) {
	simpleFeed := &RegisterFeed{
		FeedID:             1,
		FeedName:           "A Soccer Match",
		RewardPerRound:     10,
		RewardVaultInitial: 10000,
		MinDeposit:         100000,
		AppealEffect:       0,
		AppealMaxDelay:     0,
		FinalizeInterval:   5000, // 5000 ms
		ProgramID:          programs.BinaryAggregatorProgramID,
		Memo:               []byte("some random memo"),
	}
	simpleFeedRaw, err := simpleFeed.Marshal()
	require.NoError(t, err)

	testActor := codec.CreateAddress(0, ids.GenerateTestID())
	initialActorBalance := uint64(1e10)
	tests := []chaintest.ActionTest{
		{
			Name:   "SimpleRegistration",
			Actor:  testActor,
			Action: simpleFeed,
			State: func() state.Mutable {
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.SetBalance(ctx, store, testActor, initialActorBalance)
				require.NoError(t, err)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				simpleFeedStorageRaw, err := storage.GetFeed(ctx, m, uint64(1))
				require.NoError(t, err)
				require.Equal(t, simpleFeedRaw, simpleFeedStorageRaw)
				vaultValue, err := storage.GetFeedRewardVault(ctx, m, simpleFeed.FeedID)
				require.NoError(t, err)
				require.Equal(t, simpleFeed.RewardVaultInitial, vaultValue)
				actorBalance, err := storage.GetBalance(ctx, m, testActor)
				require.NoError(t, err)
				require.Equal(t, initialActorBalance-simpleFeed.RewardVaultInitial, actorBalance)
			},
			ExpectedOutputs: &RegisterFeedResult{
				FeedID: 1,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
