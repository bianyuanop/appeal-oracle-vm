package programs

import (
	"fmt"
	"slices"

	"golang.org/x/exp/maps"
)

var _ GeneralFeed = (*BinaryFeed)(nil)

type BinaryFeed struct {
	value uint8

	initialized bool
}

func NewBinaryFeed() *BinaryFeed {
	return &BinaryFeed{
		initialized: false,
	}
}

func (bf *BinaryFeed) FromRaw(raw []byte) error {
	if len(raw) != 1 {
		return fmt.Errorf("non 1 byte binary feed provided, wanted: 1, actual: %d", len(raw))
	}

	bf.value = raw[0]
	bf.initialized = true
	return nil
}

func (bf *BinaryFeed) Value() ([]byte, error) {
	ret := make([]byte, 1)
	ret[0] = bf.value
	return ret, nil
}

func GeneralFeedToBinaryFeed(feed GeneralFeed) (*BinaryFeed, error) {
	binFeed, ok := feed.(*BinaryFeed)
	if !ok {
		return nil, fmt.Errorf("unable to reflect BinaryFeed")
	}
	if !binFeed.initialized {
		return nil, fmt.Errorf("not initialized")
	}

	return binFeed, nil
}

var _ Aggregator = (*BinaryAggregator)(nil)

type BinaryAggregator struct {
	feeds    []*BinaryFeed
	majority *BinaryFeed
	tie      bool
}

func NewBinaryAggregator() *BinaryAggregator {
	return &BinaryAggregator{
		feeds:    make([]*BinaryFeed, 0),
		majority: nil,
		tie:      false,
	}
}

func (agg *BinaryAggregator) InsertFeed(feed GeneralFeed) error {
	binFeed, err := GeneralFeedToBinaryFeed(feed)
	if err != nil {
		return err
	}

	agg.feeds = append(agg.feeds, binFeed)
	return nil
}

func (agg *BinaryAggregator) CalculateMajority() {
	if len(agg.feeds) == 0 {
		return
	}

	count := make(map[uint8]int)
	for _, feed := range agg.feeds {
		count[feed.value]++
	}

	countValues := maps.Values(count)
	slices.Sort(countValues)
	n := len(countValues)
	if n >= 2 {
		if countValues[n-1] == countValues[n-2] {
			// tie
			agg.tie = true
			return
		}
	}

	biggestFeedVal := agg.feeds[0].value
	biggestFeedCnt := count[biggestFeedVal]
	for feedVal, cnt := range count {
		if cnt > biggestFeedCnt {
			biggestFeedVal = feedVal
			biggestFeedCnt = cnt
		}
	}

	agg.majority = &BinaryFeed{
		value: biggestFeedVal,
	}
}

func (agg *BinaryAggregator) Majority() GeneralFeed {
	return agg.majority
}

func (agg *BinaryAggregator) IsMajority(feed GeneralFeed) (bool, error) {
	binFeed, err := GeneralFeedToBinaryFeed(feed)
	if err != nil {
		return false, err
	}

	return agg.majority != nil && agg.majority.value == binFeed.value, nil
}

func (agg *BinaryAggregator) ProgramID() uint64 {
	return BinaryAggregatorProgramID
}
