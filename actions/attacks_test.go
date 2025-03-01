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

// test feed config
var (
	feedID           = uint64(0)
	minDeposit       = uint64(1e10)
	appealEffect     = (10 * time.Second).Milliseconds()
	maxAppealDelay   = (1 * time.Minute).Milliseconds()
	finalizeInterval = (5 * time.Second).Milliseconds()
	initialRound     = uint64(0)

	numTestAccounts = 100
	initialBalance  = uint64(1e15)
	correctValue    = []byte{1}
	wrongValue      = []byte{0}
)

func setupFeed(t *testing.T, mu state.Mutable, roundStartTime int64) {
	ctx := context.TODO()
	simpleFeed := &RegisterFeed{
		FeedID:           feedID,
		FeedName:         "A Soccer Match",
		MinDeposit:       minDeposit,
		AppealEffect:     appealEffect,
		AppealMaxDelay:   maxAppealDelay,
		FinalizeInterval: finalizeInterval, // 5000 ms
		ProgramID:        programs.BinaryAggregatorProgramID,
		Memo:             []byte("some random memo"),
	}
	simpleFeedRaw, err := simpleFeed.Marshal()
	require.NoError(t, err)

	err = storage.SetFeed(ctx, mu, simpleFeed.FeedID, simpleFeedRaw)
	require.NoError(t, err)

	err = storage.SetFeedRound(ctx, mu, feedID, &common.RoundInfo{
		RoundNumber: initialRound,
		Start:       roundStartTime,
		End:         roundStartTime + finalizeInterval,
		Delay:       0,
		Appeals:     nil,
	})
	require.NoError(t, err)
}

func deposit(t *testing.T, mu state.Mutable, actor codec.Address) {
	ctx := context.TODO()
	err := storage.SetFeedDeposit(ctx, mu, feedID, actor, minDeposit)
	require.NoError(t, err)
}

func report(t *testing.T, mu state.Mutable, actor codec.Address, value []byte) {
	ctx := context.TODO()
	err := storage.AddReportAddress(ctx, mu, initialRound, feedID, actor)
	require.NoError(t, err)
	err = storage.SetReport(ctx, mu, feedID, initialRound, actor, value)
	require.NoError(t, err)
}

func testAccounts(t *testing.T, mu state.Mutable) []codec.Address {
	ctx := context.TODO()
	ret := make([]codec.Address, 0, numTestAccounts)
	for i := 0; i < numTestAccounts; i++ {
		addr := codec.CreateAddress(0, ids.GenerateTestID())
		ret = append(ret, addr)
		// setup balance
		err := storage.SetBalance(ctx, mu, addr, initialBalance)
		require.NoError(t, err)
	}
	return ret
}

// Sybil Attacks, i.e. Collusion Attack that controls over >50% of total voting power, hence it will success but with substantial cost
func TestSybilAttack(t *testing.T) {
	actorFinalize := codec.CreateAddress(0, ids.GenerateTestID())
	finalizeReport := ReportFeed{
		FeedID: feedID,
		Value:  correctValue,
		Round:  initialRound,
	}

	roundStartTime := time.Now().UnixMilli()
	roundEndTime := roundStartTime + finalizeInterval + 1

	// setup state
	state := chaintest.NewInMemoryStore()
	setupFeed(t, state, roundStartTime)
	accounts := testAccounts(t, state)
	for i := 0; i < len(accounts); i++ {
		deposit(t, state, accounts[i])
	}
	// finalize actor deposit
	deposit(t, state, actorFinalize)
	// report feeds
	idx2split := int(float32(len(accounts))*0.5) + 1 // before this index, accounts are malicious, after are benign
	for i := 0; i < idx2split; i++ {
		report(t, state, accounts[i], wrongValue)
	}
	for j := idx2split; j < len(accounts); j++ {
		report(t, state, accounts[j], correctValue)
	}

	tests := []chaintest.ActionTest{
		{
			Name:      "SybilAttack",
			Actor:     actorFinalize,
			Timestamp: roundEndTime,
			State:     state,
			Action:    &finalizeReport,
			ExpectedOutputs: &ReportFeedResult{
				FeedID:           feedID,
				Majority:         wrongValue,
				Round:            initialRound,
				BribesFullfilled: nil,
				Sealing:          true,
			},
		},
	}
	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

// Bribery attack is similar to collusion attack but the amount of voting power is attributed by providing
// bribes, the total bribe to compromise the system is >50% deposit + half of the rewards given no appeal
func TestCommonBriberyAttack(t *testing.T) {
	actorFinalize := codec.CreateAddress(0, ids.GenerateTestID())
	finalizeReport := ReportFeed{
		FeedID: feedID,
		Value:  correctValue,
		Round:  initialRound,
	}

	roundStartTime := time.Now().UnixMilli()
	roundEndTime := roundStartTime + finalizeInterval + 1

	// setup state
	state := chaintest.NewInMemoryStore()
	setupFeed(t, state, roundStartTime)
	accounts := testAccounts(t, state)
	for i := 0; i < len(accounts); i++ {
		deposit(t, state, accounts[i])
	}
	// finalize actor deposit
	deposit(t, state, actorFinalize)
	// report feeds
	idx2split := int(float32(len(accounts))*0.5) - 1 // before this index, accounts are malicious, after are benign
	for i := 0; i < idx2split; i++ {
		report(t, state, accounts[i], wrongValue)
	}
	for j := idx2split; j < len(accounts); j++ {
		report(t, state, accounts[j], correctValue)
	}

	tests := []chaintest.ActionTest{
		{
			Name:      "CollusionAttack",
			Actor:     actorFinalize,
			Timestamp: roundEndTime,
			State:     state,
			Action:    &finalizeReport,
			ExpectedOutputs: &ReportFeedResult{
				FeedID:           feedID,
				Majority:         correctValue,
				Round:            initialRound,
				BribesFullfilled: nil,
				Sealing:          true,
			},
		},
	}
	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

// Collusion with less <50% voting power
func TestCollusionAttack(t *testing.T) {
	actorFinalize := codec.CreateAddress(0, ids.GenerateTestID())
	finalizeReport := ReportFeed{
		FeedID: feedID,
		Value:  correctValue,
		Round:  initialRound,
	}

	roundStartTime := time.Now().UnixMilli()
	roundEndTime := roundStartTime + finalizeInterval + 1

	// setup state
	state := chaintest.NewInMemoryStore()
	setupFeed(t, state, roundStartTime)
	accounts := testAccounts(t, state)
	for i := 0; i < len(accounts); i++ {
		deposit(t, state, accounts[i])
	}
	// finalize actor deposit
	deposit(t, state, actorFinalize)
	// report feeds
	idx2split := int(float32(len(accounts))*0.5) - 1 // before this index, accounts are malicious, after are benign
	for i := 0; i < idx2split; i++ {
		report(t, state, accounts[i], wrongValue)
	}
	for j := idx2split; j < len(accounts); j++ {
		report(t, state, accounts[j], correctValue)
	}

	tests := []chaintest.ActionTest{
		{
			Name:      "CollusionAttack",
			Actor:     actorFinalize,
			Timestamp: roundEndTime,
			State:     state,
			Action:    &finalizeReport,
			ExpectedOutputs: &ReportFeedResult{
				FeedID:           feedID,
				Majority:         correctValue,
				Round:            initialRound,
				BribesFullfilled: nil,
				Sealing:          true,
			},
		},
	}
	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
