package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk-starter-kit/programs"
	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/stretchr/testify/require"
)

func TestReport(t *testing.T) {
	simpleFeed := &RegisterFeed{
		FeedID:           0,
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

	tests := []chaintest.ActionTest{
		{
			Name:   "SimpleRegistration",
			Actor:  codec.EmptyAddress,
			Action: simpleFeed,
			State: func() state.Mutable {
				store := chaintest.NewInMemoryStore()
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				simpleFeedStorageRaw, err := storage.GetFeed(ctx, m, uint64(0))
				require.NoError(t, err)
				require.Equal(t, simpleFeedRaw, simpleFeedStorageRaw)
			},
			ExpectedOutputs: &RegisterFeedResult{
				FeedID: 0,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
