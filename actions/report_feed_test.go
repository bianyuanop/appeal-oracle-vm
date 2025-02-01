package actions

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk-starter-kit/programs"
	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/stretchr/testify/require"
)

func TestReportIntoANonExistsFeed(t *testing.T) {
	tests := []chaintest.ActionTest{
		{
			Name:  "ReportNonExists",
			Actor: codec.EmptyAddress,
			Action: &ReportFeed{
				FeedID: 100,
				Value:  []byte{10}, // uint8(10)
				Round:  0,
			},
			State: func() state.Mutable {
				store := chaintest.NewInMemoryStore()
				return store
			}(),
			ExpectedErr: ErrReportFeedGreaterThanHighest,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func TestReportIntoExists(t *testing.T) {
	simpleFeed := &RegisterFeed{
		FeedID:           0,
		FeedName:         "A Soccer Match",
		MinDeposit:       0,
		AppealEffect:     0,
		AppealMaxDelay:   0,
		FinalizeInterval: 5000, // 5000 ms
		ProgramID:        programs.BinaryAggregatorProgramID,
		Memo:             []byte("some random memo"),
	}
	simpleFeedRaw, err := simpleFeed.Marshal()
	require.NoError(t, err)

	testActor1 := codec.CreateAddress(0, ids.GenerateTestID())
	r1 := &ReportFeed{
		FeedID: 0,
		Value:  []byte{10}, // uint8(10)
		Round:  1,
	}

	testActor2 := codec.CreateAddress(0, ids.GenerateTestID())
	r2 := &ReportFeed{
		FeedID: 0,
		Value:  []byte{9}, // uint8(9)
		Round:  1,
	}
	testActor3 := codec.CreateAddress(0, ids.GenerateTestID())
	r3 := &ReportFeed{
		FeedID: 0,
		Value:  []byte{9}, // uint8(9)
		Round:  1,
	}
	executionTime := time.Now().UnixMilli()
	tests := []chaintest.ActionTest{
		{
			Name:      "ReportFeedNeverReported",
			Actor:     testActor1,
			Action:    r1,
			Timestamp: executionTime,
			State: func() state.Mutable {
				// set up feed in state
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.IncrementFeedID(ctx, store)
				require.NoError(t, err)
				err = storage.SetFeed(ctx, store, uint64(0), simpleFeedRaw)
				require.NoError(t, err)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				feedRoundInfo, err := storage.GetFeedRound(ctx, m, r1.FeedID)
				require.NoError(t, err)
				require.Equal(t, r1.Round, feedRoundInfo.RoundNumber)
				feedResult, err := storage.GetFeedResult(ctx, m, r1.FeedID, feedRoundInfo.RoundNumber)
				require.NoError(t, err)
				require.Equal(t, r1.Value, feedResult)
				reportAddresses, err := storage.GetReportAddresses(ctx, m, r1.Round, r1.FeedID)
				require.NoError(t, err)
				require.Equal(t, 1, len(reportAddresses))
				require.Equal(t, testActor1, reportAddresses[0])
			},
			ExpectedOutputs: &ReportFeedResult{
				FeedID:   r1.FeedID,
				Majority: r1.Value,
				Sealing:  false,
			},
		},
		{
			Name:   "ReportFeedReported",
			Actor:  testActor3,
			Action: r3,
			State: func() state.Mutable {
				// set up feed in state
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.IncrementFeedID(ctx, store)
				require.NoError(t, err)
				err = storage.SetFeed(ctx, store, uint64(0), simpleFeedRaw)
				require.NoError(t, err)
				// suppose there are two reports and they are different
				err = storage.SetReport(ctx, store, r1.FeedID, r1.Round, testActor1, r1.Value)
				require.NoError(t, err)
				err = storage.SetReport(ctx, store, r1.FeedID, r1.Round, testActor2, r2.Value)
				require.NoError(t, err)
				err = storage.SetReportAddresses(ctx, store, r1.Round, r1.FeedID, []codec.Address{testActor1, testActor2})
				require.NoError(t, err)
				// setup last result
				err = storage.SetFeedResult(ctx, store, r1.FeedID, r1.Round, r1.Value)
				require.NoError(t, err)
				return store
			}(),
			Timestamp: executionTime,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				// check report indexes are stored
				expectedRereportAddrs := []codec.Address{testActor1, testActor2, testActor3}
				reportAddrs, err := storage.GetReportAddresses(ctx, m, r1.Round, r1.FeedID)
				require.NoError(t, err)
				require.Equal(t, len(expectedRereportAddrs), len(reportAddrs))
				require.Equal(t, expectedRereportAddrs, reportAddrs)
				// check r3 report is stored
				r3ReportValue, err := storage.GetReport(ctx, m, r1.FeedID, r1.Round, testActor3)
				require.NoError(t, err)
				require.Equal(t, r3.Value, r3ReportValue)
				// check result is correct
				feedResult, err := storage.GetFeedResult(ctx, m, r1.FeedID, r1.Round)
				require.NoError(t, err)
				require.Equal(t, feedResult, r3.Value)
			},
			ExpectedOutputs: &ReportFeedResult{
				FeedID:   uint64(0),
				Majority: []byte{9},
				Sealing:  false,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func TestReportFeedWithManyReports(t *testing.T) {
	feedID := uint64(0)
	simpleFeed := &RegisterFeed{
		FeedID:           feedID,
		FeedName:         "A Soccer Match",
		MinDeposit:       0,
		AppealEffect:     0,
		AppealMaxDelay:   0,
		FinalizeInterval: 5000, // 5000 ms
		ProgramID:        programs.BinaryAggregatorProgramID,
		Memo:             []byte("some random memo"),
	}
	simpleFeedRaw, err := simpleFeed.Marshal()
	require.NoError(t, err)

	executionTime := time.Now().UnixMilli()
	r1 := &ReportFeed{
		FeedID: feedID,
		Value:  []byte{10}, // uint8(10)
		Round:  1,
	}

	testActor := codec.CreateAddress(0, ids.GenerateTestID())

	valuePositive := []byte{1}
	valueNegative := []byte{0}
	numPostiveFeeds := 100
	numNegativeFeeds := 20
	var addrsAtCur []codec.Address
	prepareHistoryReports := func(store state.Mutable) {
		ctx := context.TODO()
		addrs := make([]codec.Address, 0)
		for i := 0; i < numPostiveFeeds; i++ {
			addrID := ids.GenerateTestID()
			addr := codec.CreateAddress(0, addrID)
			addrs = append(addrs, addr)

			err := storage.SetReport(ctx, store, feedID, r1.Round, addr, valuePositive)
			require.NoError(t, err)
		}
		for i := 0; i < numNegativeFeeds; i++ {
			addrID := ids.GenerateTestID()
			addr := codec.CreateAddress(0, addrID)
			addrs = append(addrs, addr)

			err := storage.SetReport(ctx, store, feedID, r1.Round, addr, valueNegative)
			require.NoError(t, err)
		}

		err = storage.SetReportAddresses(ctx, store, r1.Round, feedID, addrs)
		require.NoError(t, err)

		addrsAtCur = addrs
	}

	tests := []chaintest.ActionTest{
		{
			Name:      "ReportFeedWithManyReports",
			Actor:     testActor,
			Action:    r1,
			Timestamp: executionTime,
			State: func() state.Mutable {
				// set up feed in state
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.IncrementFeedID(ctx, store)
				require.NoError(t, err)
				err = storage.SetFeed(ctx, store, uint64(0), simpleFeedRaw)
				require.NoError(t, err)

				prepareHistoryReports(store)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				// check report addresses are stored
				storageReportAddrs, err := storage.GetReportAddresses(ctx, m, r1.Round, r1.FeedID)
				require.NoError(t, err)
				require.Equal(t, len(storageReportAddrs), len(addrsAtCur)+1)
				addrsAtCur = append(addrsAtCur, testActor)
				require.Equal(t, storageReportAddrs, addrsAtCur)
				// check this report is stored
				storageReportValue, err := storage.GetReport(ctx, m, r1.FeedID, r1.Round, testActor)
				require.NoError(t, err)
				require.Equal(t, r1.Value, storageReportValue)
				// check feed results
				feedResult, err := storage.GetFeedResult(ctx, m, r1.FeedID, r1.Round)
				require.NoError(t, err)
				require.Equal(t, valuePositive, feedResult)
			},
			ExpectedOutputs: &ReportFeedResult{
				FeedID:   feedID,
				Majority: valuePositive,
				Sealing:  false,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
