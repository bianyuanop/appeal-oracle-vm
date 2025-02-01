package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	mconsts "github.com/ava-labs/hypersdk-starter-kit/consts"
	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var (
	ErrFundFeedNotExists = errors.New("funding feed not exists")
)

type FundFeed struct {
	FeedID uint64 `serialize:"true" json:"feedID"`
	Amount uint64 `serialize:"true" json:"amount"`
}

func (*FundFeed) GetTypeID() uint8 {
	return mconsts.RegisterFeedID
}

func (rf *FundFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.FeedIDKey()):                   state.All,
		string(storage.FeedRewardVaultKey(rf.FeedID)): state.Write | state.Read,
	}
}

func (rf *FundFeed) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	highestFeedID, err := storage.GetHighestFeedID(ctx, mu)
	if err != nil {
		return nil, err
	}
	if highestFeedID == 0 || rf.FeedID > highestFeedID {
		return nil, ErrFundFeedNotExists
	}

	// add the fund into the reward vault
	if _, err := storage.SubBalance(ctx, mu, actor, rf.Amount); err != nil {
		return nil, err
	}
	prevAmount, err := storage.GetFeedRewardVault(ctx, mu, rf.FeedID)
	if err != nil {
		return nil, err
	}
	if err := storage.SetFeedRewardVault(ctx, mu, rf.FeedID, rf.Amount+prevAmount); err != nil {
		return nil, err
	}

	return &FundFeedResult{
		FeedID: rf.FeedID,
		Amount: rf.Amount,
	}, nil
}

func (*FundFeed) ComputeUnits(chain.Rules) uint64 {
	return RegisterComputeUnit
}

func (*FundFeed) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*FundFeedResult)(nil)

type FundFeedResult struct {
	FeedID uint64 `serialize:"true" json:"feedID"`
	Amount uint64 `serialize:"true" json:"amount"`
}

func (*FundFeedResult) GetTypeID() uint8 {
	return mconsts.FundFeedID // Common practice is to use the action ID
}
