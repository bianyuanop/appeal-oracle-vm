package actions

import (
	"context"
	"errors"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	mconsts "github.com/bianyuanop/oraclevm/consts"
	"github.com/bianyuanop/oraclevm/storage"
)

const (
	DepositComputeUnit = 1
)

var (
	ErrDepositFeedGreaterThanHighest = errors.New("depositing feedID is greater than highest")
	ErrBalanceBelowDepositAmount     = errors.New("balance less than deposit amount")
)

var (
	_ chain.Action = (*DepositFeed)(nil)
)

// A fee need to be paid for the feed registration
type DepositFeed struct {
	FeedID uint64 `serialize:"true" json:"feedID"`
	Amount uint64 `serialize:"true" json:"amount"`
}

func (*DepositFeed) GetTypeID() uint8 {
	return mconsts.DepositFeedID
}

func (rf *DepositFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.FeedKey(rf.FeedID)):               state.Read,
		string(storage.FeedDepositKey(rf.FeedID, actor)): state.All,
		string(storage.BalanceKey(actor)):                state.Read | state.Write,
	}
}

func (rf *DepositFeed) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	feedInfoRaw, err := storage.GetFeed(ctx, mu, rf.FeedID)
	if err != nil {
		return nil, err
	}

	if feedInfoRaw == nil {
		return nil, ErrFeedNotExists
	}

	if _, err := storage.SubBalance(ctx, mu, actor, rf.Amount); err != nil {
		if strings.Contains(err.Error(), "invalid balance: could not subtract balance") {
			return nil, ErrBalanceBelowDepositAmount
		}
		return nil, err
	}

	if err := storage.SetFeedDeposit(ctx, mu, rf.FeedID, actor, rf.Amount); err != nil {
		return nil, err
	}

	return &DepositFeedResult{
		FeedID:  rf.FeedID,
		Amount:  rf.Amount,
		Address: actor,
	}, nil
}

func (*DepositFeed) ComputeUnits(chain.Rules) uint64 {
	return RegisterComputeUnit
}

func (*DepositFeed) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*DepositFeedResult)(nil)

type DepositFeedResult struct {
	FeedID  uint64        `serialize:"true" json:"feedID"`
	Amount  uint64        `serialize:"true" json:"amount"`
	Address codec.Address `serialize:"true" json:"address"`
}

func (*DepositFeedResult) GetTypeID() uint8 {
	return mconsts.DepositFeedID // Common practice is to use the action ID
}
