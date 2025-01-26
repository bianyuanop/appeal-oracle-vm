package actions

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk-starter-kit/programs"
	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk-starter-kit/utils"
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
				FeedID:     100,
				Value:      []byte{10}, // uint8(10)
				SubmitAt:   time.Now().UnixMilli(),
				ValidUntil: time.Now().Add(10 * time.Second).UnixMilli(),
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

	r1 := &ReportFeed{
		FeedID:     0,
		Value:      []byte{10}, // uint8(10)
		SubmitAt:   time.Now().UnixMilli(),
		ValidUntil: time.Now().Add(10 * time.Second).UnixMilli(),
	}

	testActor2 := codec.CreateAddress(0, ids.GenerateTestID())
	r2 := &ReportFeed{
		FeedID:     0,
		Value:      []byte{9}, // uint8(9)
		SubmitAt:   time.Now().UnixMilli(),
		ValidUntil: time.Now().Add(10 * time.Second).UnixMilli(),
	}
	currTime := time.Now().UnixMilli()

	testActor3 := codec.CreateAddress(0, ids.GenerateTestID())
	r3 := &ReportFeed{
		FeedID:     0,
		Value:      []byte{8}, // uint8(9)
		SubmitAt:   time.Now().UnixMilli(),
		ValidUntil: time.Now().Add(10 * time.Second).UnixMilli(),
	}
	lastResult := FeedResult{
		Value:       []byte{6},
		CreatedAt:   time.Now().UnixMilli() - 1024,
		UpdatedAt:   time.Now().UnixMilli() - 1024,
		FinalizedAt: 0,
	}

	tests := []chaintest.ActionTest{
		{
			Name:   "ReportFeedNeverReported",
			Actor:  codec.EmptyAddress,
			Action: r1,
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
				lastResultTime, err := storage.GetLastFeedResultTime(ctx, m, uint64(0))
				require.NoError(t, err)
				require.NotEqual(t, uint64(0), lastResultTime)
			},
			ExpectedOutputs: &ReportFeedResult{
				FirstTime: true,
			},
		},
		{
			Name:   "ReportFeedInitialized",
			Actor:  testActor2,
			Action: r2,
			State: func() state.Mutable {
				// set up feed in state
				ctx := context.TODO()
				store := chaintest.NewInMemoryStore()
				err := storage.IncrementFeedID(ctx, store)
				require.NoError(t, err)
				err = storage.SetFeed(ctx, store, uint64(0), simpleFeedRaw)
				require.NoError(t, err)
				err = storage.SetLastFeedResultTime(ctx, store, uint64(0), time.Now().Add(-1*time.Second).UnixMilli()/1e3)
				require.NoError(t, err)
				return store
			}(),
			Timestamp: currTime,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				lastResultTime, err := storage.GetLastFeedResultTime(ctx, m, uint64(0))
				require.NoError(t, err)
				require.NotEqual(t, uint64(0), lastResultTime)

				// check report indexes are stored
				reportAddrs := []codec.Address{testActor2}
				rawStorageReportAddrs, err := storage.GetReportIndex(ctx, m, r2.SubmitAt/1e3, r2.FeedID)
				require.NoError(t, err)
				storageReportAddrs, err := utils.DecodeAddresses(rawStorageReportAddrs)
				require.NoError(t, err)
				require.Equal(t, storageReportAddrs, reportAddrs)
				// check this report is stored
				storageReportValue, err := storage.GetReport(ctx, m, r2.FeedID, r2.SubmitAt/1e3, testActor2)
				require.NoError(t, err)
				require.Equal(t, r2.Value, storageReportValue)
				// check feed results
				expectedResults := []*FeedResult{
					{
						Value:       []byte{9},
						UpdatedAt:   currTime,
						CreatedAt:   currTime,
						FinalizedAt: 0,
					},
				}
				rawResults, err := storage.GetFeedResult(ctx, m, r2.FeedID)
				require.NoError(t, err)
				results, err := UnmarshalFeedResults(rawResults)
				require.NoError(t, err)
				require.Equal(t, expectedResults, results)
			},
			ExpectedOutputs: &ReportFeedResult{
				FeedID:    uint64(0),
				Majority:  []byte{9},
				FirstTime: false,
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
				// setup last result
				err = storage.SetLastFeedResultTime(ctx, store, uint64(0), time.Now().Add(-1*time.Second).UnixMilli()/1e3)
				require.NoError(t, err)
				results := []*FeedResult{&lastResult}
				oResults, err := MarshalFeedResults(results)
				require.NoError(t, err)
				err = storage.SetFeedResult(ctx, store, r3.FeedID, oResults)
				require.NoError(t, err)
				return store
			}(),
			Timestamp: currTime,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				lastResultTime, err := storage.GetLastFeedResultTime(ctx, m, uint64(0))
				require.NoError(t, err)
				require.NotEqual(t, uint64(0), lastResultTime)

				// check report indexes are stored
				reportAddrs := []codec.Address{testActor3}
				rawStorageReportAddrs, err := storage.GetReportIndex(ctx, m, r3.SubmitAt/1e3, r3.FeedID)
				require.NoError(t, err)
				storageReportAddrs, err := utils.DecodeAddresses(rawStorageReportAddrs)
				require.NoError(t, err)
				require.Equal(t, storageReportAddrs, reportAddrs)
				// check this report is stored
				storageReportValue, err := storage.GetReport(ctx, m, r3.FeedID, r3.SubmitAt/1e3, testActor3)
				require.NoError(t, err)
				require.Equal(t, r3.Value, storageReportValue)
				// check feed results
				expectedResults := []*FeedResult{
					{
						Value:       r3.Value,
						UpdatedAt:   currTime,
						CreatedAt:   lastResult.CreatedAt,
						FinalizedAt: 0,
					},
				}
				rawResults, err := storage.GetFeedResult(ctx, m, r3.FeedID)
				require.NoError(t, err)
				results, err := UnmarshalFeedResults(rawResults)
				require.NoError(t, err)
				require.Equal(t, expectedResults, results)
			},
			ExpectedOutputs: &ReportFeedResult{
				FeedID:    uint64(0),
				Majority:  []byte{8},
				FirstTime: false,
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

	lastResultTime := time.Now().Add(-2*time.Second).UnixMilli() / 1e3
	currTime := lastResultTime + 2
	r1 := &ReportFeed{
		FeedID:     feedID,
		Value:      []byte{10}, // uint8(10)
		SubmitAt:   time.Now().UnixMilli(),
		ValidUntil: time.Now().Add(10 * time.Second).UnixMilli(),
	}

	testActor := codec.CreateAddress(0, ids.GenerateTestID())

	valuePositive := []byte{1}
	valueNegative := []byte{0}
	numPostiveFeeds := 100
	numNegativeFeeds := 20
	var addrsAtCur []codec.Address
	prepareHistoryReports := func(store state.Mutable) {
		ctx := context.TODO()
		for ts := lastResultTime + 1; ts <= currTime; ts++ {
			addrs := make([]codec.Address, 0)
			for i := 0; i < numPostiveFeeds; i++ {
				addrID := ids.GenerateTestID()
				addr := codec.CreateAddress(0, addrID)
				addrs = append(addrs, addr)

				err := storage.SetReport(ctx, store, feedID, ts, addr, valuePositive)
				require.NoError(t, err)
			}
			for i := 0; i < numNegativeFeeds; i++ {
				addrID := ids.GenerateTestID()
				addr := codec.CreateAddress(0, addrID)
				addrs = append(addrs, addr)

				err := storage.SetReport(ctx, store, feedID, ts, addr, valueNegative)
				require.NoError(t, err)
			}

			rawAddrs, err := utils.EncodeAddresses(addrs)
			require.NoError(t, err)
			err = storage.SetReportIndex(ctx, store, ts, feedID, rawAddrs)
			require.NoError(t, err)

			if ts == currTime {
				addrsAtCur = addrs
			}
		}
	}

	executionTime := time.Now().UnixMilli()

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
				err = storage.SetLastFeedResultTime(ctx, store, uint64(0), lastResultTime)
				require.NoError(t, err)

				prepareHistoryReports(store)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				// check report indexes are stored
				// reportAddrs := []codec.Address{testActor}
				rawStorageReportAddrs, err := storage.GetReportIndex(ctx, m, r1.SubmitAt/1e3, r1.FeedID)
				require.NoError(t, err)
				storageReportAddrs, err := utils.DecodeAddresses(rawStorageReportAddrs)
				require.NoError(t, err)
				require.Equal(t, len(storageReportAddrs), len(addrsAtCur)+1)
				// check this report is stored
				storageReportValue, err := storage.GetReport(ctx, m, r1.FeedID, r1.SubmitAt/1e3, testActor)
				require.NoError(t, err)
				require.Equal(t, r1.Value, storageReportValue)
				// check feed results
				expectedResults := []*FeedResult{
					{
						Value:       valuePositive,
						UpdatedAt:   executionTime,
						CreatedAt:   executionTime,
						FinalizedAt: 0,
					},
				}
				rawResults, err := storage.GetFeedResult(ctx, m, r1.FeedID)
				require.NoError(t, err)
				results, err := UnmarshalFeedResults(rawResults)
				require.NoError(t, err)
				require.Equal(t, expectedResults, results)
			},
			ExpectedOutputs: &ReportFeedResult{
				FeedID:    feedID,
				Majority:  valuePositive,
				FirstTime: false,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
