package db

import (
	"time"
)

var (
	// "progress" -> <event> -> <user> -> progress data
	bucketProgress = []byte("progress")
)

type Progress struct {
	Puzzles map[string]PuzzleProgress
}

type PuzzleProgress struct {
	Parts []PartProgress
}

type PartProgress struct {
	Time   time.Time
	Answer string
}

func (tx *Tx) Progress() *Bucket[Progress] {
	return openBucket[Progress](tx.tx, bucketProgress)
}
