package common

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

type RoundInfo struct {
	RoundNumber uint64

	// unix miliseconds
	Start int64
	End   int64
	// TODO: add appeals
}

func (r *RoundInfo) Marshal() ([]byte, error) {
	p := codec.NewWriter(256, consts.NetworkSizeLimit)
	p.PackUint64(r.RoundNumber)
	p.PackInt64(r.Start)
	p.PackInt64(r.End)

	return p.Bytes(), p.Err()
}

func UnmarshalRoundInfo(raw []byte) (*RoundInfo, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	round := p.UnpackUint64(false)
	start := p.UnpackInt64(false)
	end := p.UnpackInt64(false)

	if p.Err() != nil {
		return nil, p.Err()
	}

	return &RoundInfo{
		RoundNumber: round,
		Start:       start,
		End:         end,
	}, nil
}
