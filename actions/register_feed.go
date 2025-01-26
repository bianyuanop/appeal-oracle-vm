package actions

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk-starter-kit/storage"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk-starter-kit/consts"
)

const (
	RegisterComputeUnit = 1
	FeedMaxMemoSize     = 1024
)

var (
	ErrRequestedFeedIDNotLatest              = errors.New("requested feed ID not the latest")
	ErrFeedMemoTooLarge                      = errors.New("feed memo size too large")
	_                           chain.Action = (*RegisterFeed)(nil)
)

// A fee need to be paid for the feed registration
type RegisterFeed struct {
	FeedID     uint64 `serialize:"true" json:"feedID"`
	FeedName   string `serialize:"true" json:"feedName"`
	MinDeposit uint64 `serialize:"true" json:"minDeposit"`
	// num of miliseconds one appeal can delay the finalization
	AppealEffect int64 `serialize:"true" json:"appealEffect"`
	// max delay appeals can result
	AppealMaxDelay int64 `serialize:"true" json:"appealMaxDelay"`
	// finalize interval in mili without any appeals
	FinalizeInterval int64  `serialize:"true" json:"finalizeInterval"`
	ProgramID        uint64 `serialize:"true" json:"programID"`
	// Optional message to accompany transaction.
	Memo []byte `serialize:"true" json:"memo"`
}

func (*RegisterFeed) GetTypeID() uint8 {
	return mconsts.RegisterFeedID
}

func (rf *RegisterFeed) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.FeedIDKey()):        state.All,
		string(storage.FeedKey(rf.FeedID)): state.All,
	}
}

func (rf *RegisterFeed) Execute(
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

	if rf.FeedID != highestFeedID+1 {
		return nil, ErrRequestedFeedIDNotLatest
	}

	if len(rf.Memo) > FeedMaxMemoSize {
		return nil, ErrFeedMemoTooLarge
	}

	rfRaw, err := rf.Marshal()
	if err != nil {
		return nil, err
	}

	// set up feed and increment highest feed id
	if err := storage.SetFeed(ctx, mu, rf.FeedID, rfRaw); err != nil {
		return nil, err
	}
	if err := storage.IncrementFeedID(ctx, mu); err != nil {
		return nil, err
	}

	return &RegisterFeedResult{
		FeedID: rf.FeedID,
	}, nil
}

func (*RegisterFeed) ComputeUnits(chain.Rules) uint64 {
	return RegisterComputeUnit
}

func (*RegisterFeed) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*RegisterFeedResult)(nil)

type RegisterFeedResult struct {
	FeedID uint64 `serialize:"true" json:"feedID"`
}

func (*RegisterFeedResult) GetTypeID() uint8 {
	return mconsts.RegisterFeedID // Common practice is to use the action ID
}

func (rf *RegisterFeed) Marshal() ([]byte, error) {
	return json.Marshal(rf)
}

// TODO: to be implemented
func UnmarshalFeed(raw []byte) (*RegisterFeed, error) {
	ret := new(RegisterFeed)
	if err := json.Unmarshal(raw, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// TODO: this is triggered at every feed submission, however, admins/feed creater should submit
// a finalize feed to finalize this feed result since in this version of hypersdk, we lose controller functionality
// to customize a event at the end of each block.Accept
type FeedResult struct {
	Value       []byte `json:"value"`
	FinalizedAt int64  `json:"finalizedAt"` // in mili
	UpdatedAt   int64  `json:"updatedAt"`   // in mili
	CreatedAt   int64  `json:"createdAt"`   // in mili
	// TODO: contain appeals in here
}

func (fr *FeedResult) Marshal(p *codec.Packer) error {
	p.PackBytes(fr.Value)
	p.PackInt64(fr.FinalizedAt)
	p.PackInt64(fr.UpdatedAt)
	p.PackInt64(fr.CreatedAt)

	return p.Err()
}

func UnmarshalFeedResult(p *codec.Packer) (*FeedResult, error) {
	fr := &FeedResult{}

	p.UnpackBytes(-1, true, &fr.Value)
	fr.FinalizedAt = p.UnpackInt64(false)
	fr.UpdatedAt = p.UnpackInt64(false)
	fr.CreatedAt = p.UnpackInt64(false)

	return fr, p.Err()
}

func MarshalFeedResults(results []*FeedResult) ([]byte, error) {
	// TODO: make a const for initial cap
	p := codec.NewWriter(100, consts.NetworkSizeLimit)
	p.PackInt(uint32(len(results)))
	for _, result := range results {
		err := result.Marshal(p)
		if err != nil {
			return nil, err
		}
	}

	return p.Bytes(), p.Err()
}

func UnmarshalFeedResults(raw []byte) ([]*FeedResult, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	numResults := p.UnpackInt(false)
	ret := make([]*FeedResult, 0, numResults)
	for i := 0; i < int(numResults); i++ {
		fr, err := UnmarshalFeedResult(p)
		if err != nil {
			return nil, err
		}
		ret = append(ret, fr)
	}

	return ret, p.Err()
}
