package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/db"
	"github.com/liennie/code-and-chill/internal/puzzles"
)

func randomAvatar() string {
	return fmt.Sprintf("https://api.dicebear.com/9.x/fun-emoji/svg?seed=%d&backgroundType=gradientLinear&mouth=cute,kissHeart,lilSmile,smileLol,smileTeeth,tongueOut,wideSmile", rand.Uint64())
}

func main() {
	file := "db/cc.db"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}

	puzzles := puzzles.Load(puzzles.Config{
		Name: "Code && Chill test",
		Events: []puzzles.EventPathConfig{{
			Path:   "test",
			Config: "../cc-test/event.yaml",
		}},
	})

	ddb := db.Open(db.Config{File: file})

	userBucketKey := db.NewBucketKey[auth.User](ddb, db.BucketUser)
	progressBucketKey := db.NewBucketKey[auth.UserProgress](ddb, db.BucketProgress)

	err := ddb.Update(func(tx *db.Tx) error {
		userBucket := userBucketKey.Open(tx)
		progressBucket := progressBucketKey.Open(tx)
		eventBucket, err := progressBucket.CreateBucket(puzzles.Default.ID)
		if err != nil {
			return err
		}

		for i := 0; i < 30; i++ {
			user := &auth.User{
				Name:         fmt.Sprintf("Fake User %d", i+1),
				AvatarURL:    randomAvatar(),
				RandomAvatar: true,
			}
			userID := fmt.Sprintf("fakeuser%02d", i)
			err := userBucket.Put(userID, user)
			if err != nil {
				return err
			}

			progress := &auth.UserProgress{
				Puzzles: map[string]auth.PuzzleProgress{},
			}

			thresh := rand.Float64()
			val := 1.
			ct := time.Duration(rand.Uint64N(uint64(15 * time.Minute)))
			mt := puzzles.Default.Puzzles[0].Unlock.Add(time.Duration(rand.Uint64N(uint64(len(puzzles.Default.Puzzles) * 24 * int(time.Hour)))))
			for i, puzzle := range puzzles.Default.Puzzles {
				pval := val
				for j := range puzzle.Parts {
					if pval < thresh {
						break
					}

					ut := puzzle.Unlock
					ut = ut.Add(ct)
					if ut.Before(mt) {
						ut = mt
					}
					ut = ut.Add(time.Duration(rand.Uint64N(uint64(5+i*30+j*10) * uint64(time.Minute))))
					progress.Puzzles[puzzle.ID] = auth.PuzzleProgress{
						Parts: append(progress.Puzzles[puzzle.ID].Parts, auth.PartProgress{
							Time: ut,
						}),
					}
					pval *= 0.9
				}
				val *= 0.8
			}

			err = eventBucket.Put(userID, progress)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		panic(err)
	}
}
