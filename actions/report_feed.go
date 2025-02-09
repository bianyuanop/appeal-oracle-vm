package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/bianyuanop/oraclevm/common"
	mconsts "github.com/bianyuanop/oraclevm/consts"
	"github.com/bianyuanop/oraclevm/programs"
	"github.com/bianyuanop/oraclevm/storage"
)

const (
	MaxHistoryFeedResultsPertain = 20

	ReportComputeUnit = 1
)

var (
	ErrReportFeedGreaterThanHighest = errors.New("reporting feedID is greater than highest")
	ErrReportIntoWrongRound         = errors.New("reporting into wrong round")
	ErrReportingWithDepositBelowMin = errors.New("reporting with deposit below minimum")
	ErrFeedNotExists                = errors.New("reporting feed not exists")
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

// TODO: some keys may be unknown, when txs are executed in parallel, it may break atomicity
func (rf *ReportFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.FeedKey(rf.FeedID)):                    state.Read,
		string(storage.FeedDepositKey(rf.FeedID, actor)):      state.Read,
		string(storage.FeedRewardVaultKey(rf.FeedID)):         state.Write | state.Read,
		string(storage.ReportIndexKey(rf.FeedID, rf.Round)):   state.All,
		string(storage.ReportKey(rf.FeedID, rf.Round, actor)): state.All,
		string(storage.BalanceKey(actor)):                     state.All,
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
	rawFeedInfo, err := storage.GetFeed(ctx, mu, rf.FeedID)
	if err != nil {
		return nil, err
	}
	if rawFeedInfo == nil {
		return nil, ErrFeedNotExists
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

	deposit, err := storage.GetFeedDeposit(ctx, mu, rf.FeedID, actor)
	if err != nil {
		return nil, err
	}

	if deposit < feedInfo.MinDeposit {
		return nil, ErrReportingWithDepositBelowMin
	}

	// sealing the feed and distribute the rewards
	if timestamp > feedRound.EndAt() {
		benignAddrs := make([]codec.Address, 0)
		maliciousAddrs := make([]codec.Address, 0)
		reporterToReport := make(map[codec.Address]programs.GeneralReport)

		// add the sealing actor, the one issue this tx to the benign set
		benignAddrs = append(benignAddrs, actor)

		agg, err := programs.NewAggregator(feedInfo.FeedID)
		if err != nil {
			return nil, err
		}

		reporters, err := storage.GetReportAddresses(ctx, mu, rf.Round, rf.FeedID)
		if err != nil {
			return nil, err
		}

		// collect and aggregate all reports and calculate the majority
		for _, reporter := range reporters {
			reportValue, err := storage.GetReport(ctx, mu, rf.FeedID, rf.Round, reporter)
			if err != nil {
				return nil, err
			}
			report, err := programs.ReportFromRaw(reportValue, feedInfo.FeedID)
			if err != nil {
				return nil, err
			}
			if err := agg.InsertReport(report); err != nil {
				return nil, err
			}
			reporterToReport[reporter] = report
		}

		// TODO: handle tie
		agg.CalculateMajority()
		// discriminate benign and malicious reporters
		for _, reporter := range reporters {
			report := reporterToReport[reporter]
			isMajority, err := agg.IsMajority(report)
			if err != nil {
				return nil, err
			}
			if isMajority {
				benignAddrs = append(benignAddrs, reporter)
			} else {
				maliciousAddrs = append(maliciousAddrs, reporter)
			}
		}

		// calculate the total reward: RewardPerRound + deposits from malicious reporters
		distributed := uint64(0)
		totalReward := feedInfo.RewardPerRound
		// sub reward from vault
		if _, err := storage.SubFeedRewardVault(ctx, mu, rf.FeedID, feedInfo.RewardPerRound); err != nil {
			return nil, err
		}
		// collect from malicious actors
		for _, reporter := range maliciousAddrs {
			deposit, err := storage.RemoveFeedDeposit(ctx, mu, rf.FeedID, reporter)
			if err != nil {
				return nil, err
			}
			totalReward += deposit
		}
		// transfer the reward to the benign accounts
		for _, reporter := range benignAddrs {
			reward := totalReward / uint64(len(benignAddrs))
			if _, err := storage.AddBalance(ctx, mu, reporter, reward); err != nil {
				return nil, err
			}
			distributed += reward
		}
		// transfer the left into the vault
		if left := totalReward - distributed; left != 0 {
			_, err := storage.AddFeedRewardVault(ctx, mu, rf.FeedID, left)
			if err != nil {
				return nil, err
			}
		}

		newFeedRound := &common.RoundInfo{
			RoundNumber: feedRound.RoundNumber + 1,
			Start:       timestamp,
			End:         timestamp + feedInfo.FinalizeInterval,
		}
		// store the new round info right away
		if err := storage.SetFeedRound(ctx, mu, rf.FeedID, newFeedRound); err != nil {
			return nil, err
		}

		// store the feed result
		feedResultMajority, err := agg.Majority().Value()
		if err != nil {
			return nil, err
		}
		if err := storage.SetFeedResult(ctx, mu, rf.FeedID, rf.Round, feedResultMajority); err != nil {
			return nil, err
		}

		return &ReportFeedResult{
			FeedID:   rf.FeedID,
			Round:    rf.Round,
			Majority: feedResultMajority,
			Sealing:  true,
		}, nil
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
		Round:    rf.Round,
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
	Round    uint64 `serialize:"true" json:"round"`
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
