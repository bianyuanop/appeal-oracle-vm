package programs

import (
	"fmt"
	"slices"

	"golang.org/x/exp/maps"
)

var _ GeneralReport = (*BinaryReport)(nil)

type BinaryReport struct {
	value uint8

	initialized bool
}

func NewBinaryFeed() *BinaryReport {
	return &BinaryReport{
		initialized: false,
	}
}

func (bf *BinaryReport) FromRaw(raw []byte) error {
	if len(raw) != 1 {
		return fmt.Errorf("non 1 byte binary report provided, wanted: 1, actual: %d", len(raw))
	}

	bf.value = raw[0]
	bf.initialized = true
	return nil
}

func (bf *BinaryReport) Value() ([]byte, error) {
	ret := make([]byte, 1)
	ret[0] = bf.value
	return ret, nil
}

func GeneralFeedToBinaryFeed(report GeneralReport) (*BinaryReport, error) {
	binFeed, ok := report.(*BinaryReport)
	if !ok {
		return nil, fmt.Errorf("unable to reflect BinaryReport")
	}
	if !binFeed.initialized {
		return nil, fmt.Errorf("not initialized")
	}

	return binFeed, nil
}

var _ Aggregator = (*BinaryAggregator)(nil)

type BinaryAggregator struct {
	reports  []*BinaryReport
	majority *BinaryReport
	tie      bool
}

func NewBinaryAggregator() *BinaryAggregator {
	return &BinaryAggregator{
		reports:  make([]*BinaryReport, 0),
		majority: nil,
		tie:      false,
	}
}

func (agg *BinaryAggregator) InsertReport(report GeneralReport) error {
	binFeed, err := GeneralFeedToBinaryFeed(report)
	if err != nil {
		return err
	}

	agg.reports = append(agg.reports, binFeed)
	return nil
}

func (agg *BinaryAggregator) CalculateMajority() {
	if len(agg.reports) == 0 {
		return
	}

	count := make(map[uint8]int)
	for _, report := range agg.reports {
		count[report.value]++
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

	biggestFeedVal := agg.reports[0].value
	biggestFeedCnt := count[biggestFeedVal]
	for feedVal, cnt := range count {
		if cnt > biggestFeedCnt {
			biggestFeedVal = feedVal
			biggestFeedCnt = cnt
		}
	}

	agg.majority = &BinaryReport{
		value: biggestFeedVal,
	}
}

func (agg *BinaryAggregator) Majority() GeneralReport {
	return agg.majority
}

func (agg *BinaryAggregator) IsMajority(report GeneralReport) (bool, error) {
	binFeed, err := GeneralFeedToBinaryFeed(report)
	if err != nil {
		return false, err
	}

	return agg.majority != nil && agg.majority.value == binFeed.value, nil
}

func (agg *BinaryAggregator) ProgramID() uint64 {
	return BinaryAggregatorProgramID
}
