package common

import "errors"

var (
	ErrAppealAlreadyExists = errors.New("appeal issuer already issued appeal in this round")
)
