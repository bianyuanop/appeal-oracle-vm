package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk-starter-kit/common"
	mconsts "github.com/ava-labs/hypersdk-starter-kit/consts"
	"github.com/ava-labs/hypersdk-starter-kit/programs"
	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

const (
	MaxHistoryFeedResultsPertain = 20

	ReportComputeUnit = 1
)

var (
	ErrReportFeedGreaterThanHighest = errors.New("reporting feedID is greater than highest")
	ErrReportIntoWrongRound         = errors.New("reporting into wrong round")
	ErrReportingWithDepositBelowMin = errors.New("reporting with deposit below minimum")
)

var (
	_ chain.Action = (*ReportFeed)(nil)
)

// A fee need to be paid for the feed registration
type ReportFeed struct {
	FeedID uint64 `serialize:"true" json:"feedID"`
	Value  []byte `serialize:"true" json:"value"`
	Round  uint64 `serialize:"true" json:"round"`
}

func (*ReportFeed) GetTypeID() uint8 {
	return mconsts.ReportFeedID
}

func (rf *ReportFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.FeedKey(rf.FeedID)):                    state.Read,
		string(storage.FeedDepositKey(rf.FeedID, actor)):      state.Read,
		string(storage.ReportIndexKey(rf.FeedID, rf.Round)):   state.All,
		string(storage.ReportKey(rf.FeedID, rf.Round, actor)): state.All,
	}
}

func (rf *ReportFeed) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	highestFeedID, err := storage.GetHighestFeedID(ctx, mu)
	if err != nil {
		return nil, err
	}

	if rf.FeedID > highestFeedID {
		return nil, ErrReportFeedGreaterThanHighest
	}

	rawFeedInfo, err := storage.GetFeed(ctx, mu, rf.FeedID)
	if err != nil {
		return nil, err
	}

	feedInfo, err := UnmarshalFeed(rawFeedInfo)
	if err != nil {
		return nil, err
	}

	feedRound, err := storage.GetFeedRound(ctx, mu, rf.FeedID)
	if err != nil {
		return nil, err
	}

	// the feed is first time got reported
	if feedRound == nil {
		feedRound = &common.RoundInfo{
			RoundNumber: 1,
			Start:       timestamp,
			End:         timestamp + feedInfo.FinalizeInterval,
		}
		// store the round info right away
		if err := storage.SetFeedRound(ctx, mu, rf.FeedID, feedRound); err != nil {
			return nil, err
		}
	}

	if feedRound.RoundNumber != rf.Round {
		return nil, ErrReportIntoWrongRound
	}

	if timestamp > feedRound.End {
		// TODO: some rewards are needed for sealing
		newFeedRound := &common.RoundInfo{
			RoundNumber: feedRound.RoundNumber + 1,
			Start:       timestamp,
			End:         timestamp + feedInfo.FinalizeInterval,
		}
		// store the new round info right away
		if err := storage.SetFeedRound(ctx, mu, rf.FeedID, newFeedRound); err != nil {
			return nil, err
		}

		return &ReportFeedResult{
			FeedID:  rf.FeedID,
			Sealing: true,
		}, nil
	}

	deposit, err := storage.GetFeedDeposit(ctx, mu, rf.FeedID, actor)
	if err != nil {
		return nil, err
	}

	if deposit < feedInfo.MinDeposit {
		return nil, ErrReportingWithDepositBelowMin
	}

	agg, err := programs.NewAggregator(feedInfo.ProgramID)
	if err != nil {
		return nil, err
	}

	// fetch all report indexes for this feed in this round
	reportAddresses, err := storage.GetReportAddresses(ctx, mu, rf.Round, rf.FeedID)
	if err != nil {
		return nil, err
	}

	for _, addr := range reportAddresses {
		reportValue, err := storage.GetReport(ctx, mu, rf.FeedID, rf.Round, addr)
		if err != nil {
			return nil, err
		}

		report, err := programs.ReportFromRaw(reportValue, feedInfo.ProgramID)
		if err != nil {
			return nil, err
		}

		if err := agg.InsertReport(report); err != nil {
			return nil, err
		}
	}

	// insert current report
	currReport, err := programs.ReportFromRaw(rf.Value, feedInfo.ProgramID)
	if err != nil {
		return nil, err
	}
	if err := agg.InsertReport(currReport); err != nil {
		return nil, err
	}

	// TODO: handle tie
	// calculate the majority
	agg.CalculateMajority()

	// store the calculated results and store the report & the addresses
	feedResult := agg.Majority()
	feedResultValue, err := feedResult.Value()
	if err != nil {
		return nil, err
	}

	if err := storage.SetFeedResult(ctx, mu, rf.FeedID, rf.Round, feedResultValue); err != nil {
		return nil, err
	}

	reportAddresses = append(reportAddresses, actor)
	if err := storage.SetReportAddresses(ctx, mu, rf.Round, rf.FeedID, reportAddresses); err != nil {
		return nil, err
	}
	// store this report
	err = storage.SetReport(ctx, mu, rf.FeedID, rf.Round, actor, rf.Value)
	if err != nil {
		return nil, err
	}

	return &ReportFeedResult{
		FeedID:   rf.FeedID,
		Majority: feedResultValue,
	}, nil
}

func (*ReportFeed) ComputeUnits(chain.Rules) uint64 {
	return RegisterComputeUnit
}

func (*ReportFeed) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*ReportFeedResult)(nil)

type ReportFeedResult struct {
	FeedID   uint64 `serialize:"true" json:"feedID"`
	Majority []byte `serialize:"true" json:"majority"`
	// the last tx reports into an expired round will seal the feed at that round and increment the round number
	Sealing bool `serialize:"true" json:"sealing"`
}

func (*ReportFeedResult) GetTypeID() uint8 {
	return mconsts.ReportFeedID // Common practice is to use the action ID
}

// TODO: to be implemented
func (*ReportFeed) Marshal() ([]byte, error) {
	return nil, nil
}
