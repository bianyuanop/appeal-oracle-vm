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
	"github.com/bianyuanop/oraclevm/storage"
)

const (
	AppealComputeUnit = 1
)

var (
	ErrAppealFeedGreaterThanHighest = errors.New("appealing feedID is greater than highest")
	ErrBalanceBelowAppealFee        = errors.New("balance below feed preset appeal fee")
)

var (
	_ chain.Action = (*AppealFeed)(nil)
)

// A fee need to be paid for the feed registration
type AppealFeed struct {
	FeedID uint64 `serialize:"true" json:"feedID"`
}

func (*AppealFeed) GetTypeID() uint8 {
	return mconsts.AppealFeedID
}

func (rf *AppealFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.FeedKey(rf.FeedID)):            state.Read,
		string(storage.BalanceKey(actor)):             state.Read | state.Write,
		string(storage.FeedRoundKey(rf.FeedID)):       state.Read | state.Write,
		string(storage.FeedRewardVaultKey(rf.FeedID)): state.Read | state.Write,
	}
}

func (rf *AppealFeed) Execute(
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
		return nil, ErrDepositFeedGreaterThanHighest
	}

	feedInfoRaw, err := storage.GetFeed(ctx, mu, rf.FeedID)
	if err != nil {
		return nil, err
	}
	feedInfo, err := UnmarshalFeed(feedInfoRaw)
	if err != nil {
		return nil, err
	}

	// move appeal fee to vault
	_, err = storage.SubBalance(ctx, mu, actor, feedInfo.AppealFee)
	if err != nil {
		return nil, err
	}

	if _, err = storage.AddFeedRewardVault(ctx, mu, rf.FeedID, feedInfo.AppealFee); err != nil {
		return nil, err
	}

	// apply appeal
	roundInfo, err := storage.GetFeedRound(ctx, mu, rf.FeedID)
	if err != nil {
		return nil, err
	}
	if err := roundInfo.ApplyAppeal(feedInfo.AppealMaxDelay, feedInfo.AppealEffect, &common.AppealInfo{
		Issuer: actor,
	}); err != nil {
		return nil, err
	}

	if err := storage.SetFeedRound(ctx, mu, feedInfo.FeedID, roundInfo); err != nil {
		return nil, err
	}

	return &AppealFeedResult{
		FeedID:  rf.FeedID,
		EndTime: roundInfo.EndAt(),
	}, nil
}

func (*AppealFeed) ComputeUnits(chain.Rules) uint64 {
	return RegisterComputeUnit
}

func (*AppealFeed) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*AppealFeedResult)(nil)

type AppealFeedResult struct {
	FeedID  uint64 `serialize:"true" json:"feedID"`
	EndTime int64  `serialize:"true" json:"endTime"`
}

func (*AppealFeedResult) GetTypeID() uint8 {
	return mconsts.AppealFeedID // Common practice is to use the action ID
}

// TODO: to be implemented
func (*AppealFeed) Marshal() ([]byte, error) {
	return nil, nil
}
