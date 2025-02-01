package programs

import (
	"fmt"
)

type GeneralReport interface {
	FromRaw([]byte) error
	Value() ([]byte, error)
}

func ReportFromRaw(raw []byte, programID uint64) (GeneralReport, error) {
	switch programID {
	case BinaryAggregatorProgramID:
		feed := NewBinaryFeed()
		if err := feed.FromRaw(raw); err != nil {
			return nil, err
		}
		return feed, nil
	default:
		return nil, fmt.Errorf("unknown program: %d", programID)
	}
}

type Aggregator interface {
	CalculateMajority()
	Majority() GeneralReport
	InsertReport(report GeneralReport) error
	IsMajority(report GeneralReport) (bool, error)
	ProgramID() uint64
}

var AggregatorRegistry map[int]Aggregator

func NewAggregator(programID uint64) (Aggregator, error) {
	switch programID {
	case BinaryAggregatorProgramID:
		return NewBinaryAggregator(), nil
	default:
		return nil, fmt.Errorf("unknown program: %d", programID)
	}
}
