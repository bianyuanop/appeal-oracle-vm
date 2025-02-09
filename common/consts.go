package common

import "errors"

const (
	MaxFeedValue = 1024 // 1KB
)

var (
	ErrAppealAlreadyExists = errors.New("appeal issuer already issued appeal in this round")
	ErrBribeExists         = errors.New("bribe already exists")
	ErrBribeNotExists      = errors.New("bribe not exists")
)
