package utils

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

func EncodeAddresses(addrs []codec.Address) ([]byte, error) {
	p := codec.NewWriter(0, consts.NetworkSizeLimit)
	p.PackInt(uint32(len(addrs)))
	for _, addr := range addrs {
		p.PackAddress(addr)
	}

	return p.Bytes(), p.Err()
}

func DecodeAddresses(raw []byte) ([]codec.Address, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	numAddrs := p.UnpackInt(false)
	ret := make([]codec.Address, 0, numAddrs)
	for i := 0; i < int(numAddrs); i++ {
		var addr codec.Address
		p.UnpackAddress(&addr)
		ret = append(ret, addr)
	}

	return ret, p.Err()
}
