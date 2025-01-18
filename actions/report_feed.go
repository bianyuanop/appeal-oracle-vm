package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	mconsts "github.com/ava-labs/hypersdk-starter-kit/consts"
	"github.com/ava-labs/hypersdk-starter-kit/programs"
	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk-starter-kit/utils"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

const (
	MaxHistoryFeedResultsPertain = 20

	ReportComputeUnit = 1
)

var (
	_ chain.Action = (*ReportFeed)(nil)
)

// A fee need to be paid for the feed registration
type ReportFeed struct {
	FeedID uint64 `serialize:"true" json:"feedID"`
	Value  []byte `serialize:"true" json:"value"`
	// the reason adding this field is because we are unable to get execution time in the StateKeys
	SubmitAt   int64 `serialize:"true" json:"submitAt"`   // in mili
	ValidUntil int64 `serialize:"true" json:"validUntil"` // in mili
}

func (*ReportFeed) GetTypeID() uint8 {
	return mconsts.ReportFeedID
}

func (rf *ReportFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	submitAtInSeconds := rf.SubmitAt / 1e3

	return state.Keys{
		string(storage.FeedKey(rf.FeedID)):                             state.Read,
		string(storage.ReportIndexKey(rf.FeedID, submitAtInSeconds)):   state.All,
		string(storage.ReportKey(rf.FeedID, submitAtInSeconds, actor)): state.All,
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

	if rf.FeedID < highestFeedID {
		return nil, fmt.Errorf("reporting feedID is greater than highest")
	}

	rawFeedInfo, err := storage.GetFeed(ctx, mu, rf.FeedID)
	if err != nil {
		return nil, err
	}

	feedInfo, err := UnmarshalFeed(rawFeedInfo)
	if err != nil {
		return nil, err
	}

	// TODO: a deposit check for the actor

	// TODO: select using program ID
	agg, err := programs.NewAggregator(feedInfo.ProgramID)
	if err != nil {
		return nil, err
	}
	submitAtInSeconds := rf.SubmitAt / 1e3

	rawAddresses, err := storage.GetReportIndex(ctx, mu, submitAtInSeconds, rf.FeedID)
	if err != nil {
		return nil, err
	}
	addrs, err := utils.DecodeAddresses(rawAddresses)
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		rawFeed, err := storage.GetReport(ctx, mu, rf.FeedID, submitAtInSeconds, addr)
		if err != nil {
			return nil, err
		}
		feed, err := programs.FeedFromRaw(rawFeed, feedInfo.ProgramID)
		if err != nil {
			return nil, err
		}

		if err := agg.InsertFeed(feed); err != nil {
			return nil, err
		}
	}

	agg.CalculateMajority()

	// load the old feedResult
	var feedResults []*FeedResult
	feedResultsRaw, err := storage.GetFeedResult(ctx, mu, rf.FeedID)
	if errors.Is(err, database.ErrNotFound) {
		feedResults = nil
	} else if err != nil {
		return nil, err
	} else {
		feedResults, err = UnmarshalFeedResults(feedResultsRaw)
		if err != nil {
			return nil, err
		}
	}

	// calculate the feed result and update state, if finalized penalize wrong feed provider
	feedMajority := agg.Majority()
	majorityValue, err := feedMajority.Value()
	if err != nil {
		return nil, err
	}
	if len(feedResults) > 0 && feedResults[len(feedResults)-1].FinalizedAt == 0 {
		// previous one not yet finalized
		lastFeedResult := feedResults[len(feedResults)-1]
		lastFeedResult.Value = majorityValue
		// TODO: add appeal delay in here
		if timestamp-lastFeedResult.CreatedAt >= feedInfo.FinalizeInterval {
			lastFeedResult.FinalizedAt = timestamp
		} else {
			lastFeedResult.UpdatedAt = timestamp
		}
	} else {
		// previous one finalized or this is the first one
		feedResults = append(feedResults, &FeedResult{
			Value:     majorityValue,
			UpdatedAt: timestamp,
			CreatedAt: timestamp,
		})
	}
	// prune old ones
	if len(feedResults) > MaxHistoryFeedResultsPertain {
		feedResults = feedResults[len(feedResults)-MaxHistoryFeedResultsPertain:]
	}

	feedResultsRaw, err = MarshalFeedResults(feedResults)
	if err != nil {
		return nil, err
	}

	if err := storage.SetFeedResult(ctx, mu, rf.FeedID, feedResultsRaw); err != nil {
		return nil, err
	}

	return &ReportFeedResult{
		FeedID:   rf.FeedID,
		Majority: majorityValue,
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
}

func (*ReportFeedResult) GetTypeID() uint8 {
	return mconsts.ReportFeedID // Common practice is to use the action ID
}

// TODO: to be implemented
func (*ReportFeed) Marshal() ([]byte, error) {
	return nil, nil
}
