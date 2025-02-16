package common

import (
	"bytes"
	"slices"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

type AppealInfo struct {
	Issuer codec.Address
}

func (a *AppealInfo) Marshal(p *codec.Packer) error {
	p.PackAddress(a.Issuer)
	return p.Err()
}

func UnmarshalAppealInfo(p *codec.Packer) (*AppealInfo, error) {
	info := new(AppealInfo)
	p.UnpackAddress(&info.Issuer)
	if p.Err() != nil {
		return nil, p.Err()
	}
	return info, nil
}

type RoundInfo struct {
	RoundNumber uint64

	// unix miliseconds
	Start   int64
	End     int64
	Delay   int64
	Appeals []*AppealInfo
}

func (r *RoundInfo) EndAt() int64 {
	return r.End + r.Delay
}

// TODO: move feed info to this package to avoid circular import and elegant method signature
func (r *RoundInfo) ApplyAppeal(maxDelay int64, appealDelay int64, appeal *AppealInfo) error {
	if slices.ContainsFunc(r.Appeals, func(a *AppealInfo) bool {
		return bytes.Equal(a.Issuer[:], appeal.Issuer[:])
	}) {
		return ErrAppealAlreadyExists
	}

	r.Appeals = append(r.Appeals, appeal)

	delay := appealDelay * int64(len(r.Appeals))
	if delay > maxDelay {
		delay = maxDelay
	}
	r.Delay = delay
	return nil
}

func (r *RoundInfo) Marshal() ([]byte, error) {
	p := codec.NewWriter(256, consts.NetworkSizeLimit)
	p.PackUint64(r.RoundNumber)
	p.PackInt64(r.Start)
	p.PackInt64(r.End)
	p.PackInt64(r.Delay)
	numAppeals := len(r.Appeals)
	p.PackInt(uint32(numAppeals))
	for _, appeal := range r.Appeals {
		if err := appeal.Marshal(p); err != nil {
			return nil, err
		}
	}

	return p.Bytes(), p.Err()
}

func UnmarshalRoundInfo(raw []byte) (*RoundInfo, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	round := p.UnpackUint64(false)
	start := p.UnpackInt64(false)
	end := p.UnpackInt64(false)
	delay := p.UnpackInt64(false)
	numAppeals := p.UnpackInt(false)
	appeals := make([]*AppealInfo, 0, numAppeals)
	for i := 0; i < int(numAppeals); i++ {
		appeal, err := UnmarshalAppealInfo(p)
		if err != nil {
			return nil, err
		}
		appeals = append(appeals, appeal)
	}

	if p.Err() != nil {
		return nil, p.Err()
	}

	return &RoundInfo{
		RoundNumber: round,
		Start:       start,
		End:         end,
		Delay:       delay,
		Appeals:     appeals,
	}, nil
}

type BribeInfo struct {
	Amount   uint64        `json:"amount"`
	Provider codec.Address `json:"provider"`
	// the reporter can be rewarded only when the submitted value exactly match with reported
	Value []byte `json:"value"`
}

func (b *BribeInfo) Marshal(p *codec.Packer) error {
	p.PackUint64(b.Amount)
	p.PackAddress(b.Provider)
	p.PackBytes(b.Value)
	if p.Err() != nil {
		return p.Err()
	}
	return nil
}

func UnmarshalBribeInfo(p *codec.Packer) (*BribeInfo, error) {
	ret := new(BribeInfo)
	ret.Amount = p.UnpackUint64(false)
	p.UnpackAddress(&ret.Provider)
	p.UnpackBytes(MaxFeedValue, true, &ret.Value)
	if err := p.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

func MarshalBribeInfoArray(bribes []*BribeInfo) ([]byte, error) {
	p := codec.NewWriter(1024, consts.NetworkSizeLimit)
	p.PackInt(uint32(len(bribes)))
	for _, b := range bribes {
		if err := b.Marshal(p); err != nil {
			return nil, err
		}
	}
	if err := p.Err(); err != nil {
		return nil, err
	}
	return p.Bytes(), nil
}

func UnmarshalBribeInfoArray(raw []byte) ([]*BribeInfo, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	numBribes := p.UnpackInt(false)
	if err := p.Err(); err != nil {
		return nil, err
	}

	ret := make([]*BribeInfo, 0, numBribes)
	for i := 0; i < int(numBribes); i++ {
		info, err := UnmarshalBribeInfo(p)
		if err != nil {
			return nil, err
		}
		ret = append(ret, info)
	}
	return ret, nil
}
