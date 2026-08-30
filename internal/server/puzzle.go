package server

import (
	"cmp"
	"errors"
	"fmt"
	"hash/fnv"
	"html"
	"html/template"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/liennie/code-and-chill/internal/auth"
	"github.com/liennie/code-and-chill/internal/ctxlog"
	"github.com/liennie/code-and-chill/internal/puzzles"
)

// func eventsMiddleware(events []puzzles.Event, next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		pd := pageDataFromContext(r.Context())

// 		maxLen := 0
// 		for _, event := range events {
// 			if len(event.Name) > maxLen {
// 				maxLen = len(event.Name)
// 			}
// 		}
// 		pd.EventAlign = maxLen + 2

// 		pd.Events = make([]eventData, 0, len(events))
// 		for _, event := range events {
// 			// TODO solved from user

// 			evd := eventData{
// 				Path:  event.Path,
// 				Name:  event.Name,
// 				Total: event.Total,
// 			}

// 			pd.Events = append(pd.Events, evd)
// 		}

// 		next.ServeHTTP(w, r)
// 	})
// }

func puzzlesMiddleware(event puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		pd.Event = eventData{
			Path: event.Path,
			Name: event.Name,
		}

		maxLen := 0
		for _, puzzle := range event.Puzzles {
			if len(puzzle.Name) > maxLen {
				maxLen = len(puzzle.Name)
			}
		}
		pd.PuzzleAlign = maxLen + 2

		user := userFromContext(r.Context())
		progress := progressFromContext(r.Context())

		pd.Event.Contacts = make([]contactData, 0, len(event.Contacts))
		for _, contact := range event.Contacts {
			private := contact.Private && (user == nil || user.Hidden)
			pd.Event.Contacts = append(pd.Event.Contacts, contactData{
				Title:   contact.Title,
				Link:    contact.Link,
				Private: private,
			})
		}

		pd.Puzzles = make([]puzzleData, 0, len(event.Puzzles))
		for _, puzzle := range event.Puzzles {
			pzd := puzzleData{
				Path:  puzzle.Path,
				Name:  puzzle.Name,
				Total: len(puzzle.Parts),
			}

			if puzzle.Locked(pd.Now, user) {
				pzd.Locked = true
				pzd.Unlock = &puzzle.Unlock
			} else {
				pd.PuzzleUnlocked = true
			}

			if progress != nil {
				pzd.Solved = len(progress.Puzzles[puzzle.ID].Parts)
			}

			pd.Puzzles = append(pd.Puzzles, pzd)
		}

		next.ServeHTTP(w, r)
	})
}

func puzzleDataFunc(puzzle puzzles.Puzzle, locked, content dataFunc) dataFunc {
	return func(r *http.Request) int {
		pd := pageDataFromContext(r.Context())
		pd.Puzzle = currentPuzzleData{
			Path: puzzle.Path,
			Name: puzzle.Name,
		}
		pd.Title = puzzle.Name

		user := userFromContext(r.Context())
		if puzzle.Locked(pd.Now, user) {
			return locked(r)
		}

		progress := progressFromContext(r.Context())
		pd.Puzzle.Parts = make([]partData, 0, len(puzzle.Parts))
		for i, part := range puzzle.Parts {
			solved := false
			if progress != nil {
				solved = len(progress.Puzzles[puzzle.ID].Parts) > i
			}

			answer := ""
			if solved && user != nil {
				input := inputIndex(user.ID, puzzle)
				answer = puzzle.Inputs[input].Answers[i]
			}

			pd.Puzzle.Parts = append(pd.Puzzle.Parts, partData{
				Anchor: part.ID,
				MD:     part.Text,
				Answer: answer,
			})

			if !solved {
				break
			}
		}
		if progress != nil {
			pd.Puzzle.Finished = len(progress.Puzzles[puzzle.ID].Parts) >= len(puzzle.Parts)
		}

		return content(r)
	}
}

func latestPuzzleRedirect(event puzzles.Event) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		user := userFromContext(r.Context())

		for i := len(event.Puzzles) - 1; i >= 0; i-- {
			puzzle := event.Puzzles[i]
			if puzzle.Locked(pd.Now, user) {
				continue
			}

			http.Redirect(w, r, fmt.Sprintf("/%s/puzzle/%s", event.Path, puzzle.Path), http.StatusTemporaryRedirect)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/%s", event.Path), http.StatusTemporaryRedirect)
	})
}

func inputIndex(userID string, puzzle puzzles.Puzzle) uint {
	// TODO save input index in db
	h := fnv.New64()
	io.WriteString(h, userID)
	io.WriteString(h, puzzle.ID)
	return uint(h.Sum64()) % uint(len(puzzle.Inputs))
}

func puzzleInputHandler(puzzle puzzles.Puzzle, locked http.Handler, unauth http.Handler) http.Handler {
	handlers := make([]http.Handler, len(puzzle.Inputs))
	for i, input := range puzzle.Inputs {
		handlers[i] = cachedHandler(input.Text, "text/plain; charset=utf-8")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())
		user := userFromContext(r.Context())

		if puzzle.Locked(pd.Now, user) {
			locked.ServeHTTP(w, r)
			return
		}

		if user == nil {
			// this should never happen
			unauth.ServeHTTP(w, r)
			return
		}

		i := inputIndex(user.ID, puzzle)
		handlers[i].ServeHTTP(w, r)
	})
}

var errCancelUpdate = fmt.Errorf("cancel progress update")

type puzzleAnswerDataFuncs struct {
	locked        dataFunc
	unauth        dataFunc
	badRequest    dataFunc
	empty         dataFunc
	alreadySolved dataFunc
	badPart       dataFunc
	timeout       dataFunc
	incorrect     dataFunc
	correct       dataFunc
}

func puzzleAnswerDataFunc(a *auth.Auth, event puzzles.Event, pidx int, puzzle puzzles.Puzzle, dataFuncs puzzleAnswerDataFuncs) dataFunc {
	return func(r *http.Request) int {
		pd := pageDataFromContext(r.Context())
		pd.Puzzle = currentPuzzleData{
			Path: puzzle.Path,
			Name: puzzle.Name,
		}

		user := userFromContext(r.Context())
		if puzzle.Locked(pd.Now, user) {
			return dataFuncs.locked(r)
		}

		if user == nil {
			// this should never happen
			return dataFuncs.unauth(r)
		}

		err := r.ParseForm()
		if err != nil {
			return dataFuncs.badRequest(r)
		}

		rawPart := r.PostForm.Get("part")
		part, err := strconv.Atoi(rawPart)
		if err != nil {
			return dataFuncs.badPart(r)
		}

		ii := inputIndex(user.ID, puzzle)
		if part < 0 || part >= len(puzzle.Parts) || part >= len(puzzle.Inputs[ii].Answers) {
			return dataFuncs.badPart(r)
		}

		progress := progressFromContext(r.Context())
		if progress == nil && part != 0 {
			// quick check for first answer before we add progress with an update
			return dataFuncs.badPart(r)
		}

		var df dataFunc
		err = a.UpdateProgress(event.ID, user.ID, func(progress *auth.UserProgress) error {
			pp := progress.Puzzles[puzzle.ID]
			// second check with up to date data
			if part < len(pp.Parts) {
				part++
				if part < len(puzzle.Parts) {
					pd.Puzzle.Part = part + 1
					pd.Puzzle.Anchor = puzzle.Parts[part].ID
				} else {
					pd.Puzzle.Finished = true
				}
				df = dataFuncs.alreadySolved
				return errCancelUpdate

			} else if part > len(pp.Parts) {
				df = dataFuncs.badPart
				return errCancelUpdate
			}
			// part == len(pp.Parts)

			answer := r.PostForm.Get("answer")
			answer = strings.TrimSpace(answer)
			answer = strings.NewReplacer("\t", " ", "\n", " ", "\v", " ", "\f", " ", "\r", "").Replace(answer)
			if answer == "" {
				pd.Puzzle.Anchor = puzzle.Parts[part].ID
				df = dataFuncs.empty
				return errCancelUpdate
			}
			pd.Puzzle.Submitted = answer

			if progress.Timeout.After(pd.Now) {
				pd.Puzzle.Anchor = puzzle.Parts[part].ID
				pd.Puzzle.Timeout = progress.Timeout
				df = dataFuncs.timeout
				return errCancelUpdate
			}

			correctAnswer := puzzle.Inputs[ii].Answers[part]

			if answer != correctAnswer {
				progress.Incorrect++
				if progress.Incorrect < 5 {
					progress.Timeout = pd.Now.Add(time.Minute)
				} else {
					progress.Timeout = pd.Now.Add(5 * time.Minute)
				}

				pd.Puzzle.Correct = correctAnswer
				pd.Puzzle.Incorrect = progress.Incorrect
				pd.Puzzle.Anchor = puzzle.Parts[part].ID
				df = dataFuncs.incorrect
				return nil
			}

			progress.Incorrect = 0
			progress.Timeout = time.Time{}
			// append is fine because we check that part == len(pp.Parts) above
			pp.Parts = append(pp.Parts, auth.PartProgress{
				Time: pd.Now,
			})
			progress.Puzzles[puzzle.ID] = pp

			if pidx >= 0 && pidx < len(pd.Puzzles) {
				pd.Puzzles[pidx].Solved++
			}

			part++
			if part < len(puzzle.Parts) {
				pd.Puzzle.Part = part + 1
				pd.Puzzle.Anchor = puzzle.Parts[part].ID
			} else {
				pd.Puzzle.Finished = true
			}
			df = dataFuncs.correct
			return nil
		})
		if err != nil {
			if !errors.Is(err, errCancelUpdate) {
				panic(fmt.Errorf("update progress: %w", err))
			}
		}
		if df == nil {
			panic("no data func set in puzzle answer handler")
		}
		return df(r)
	}
}

type userProgress struct {
	user      *auth.User
	puzzles   map[string]auth.PuzzleProgress
	solved    int
	score     int
	lastSolve time.Time
}

func (p *userProgress) Compare(other *userProgress) int {
	return cmp.Or(
		-cmp.Compare(p.solved, other.solved),
		-cmp.Compare(p.score, other.score),
		p.lastSolve.Compare(other.lastSolve),
		cmp.Compare(strings.ToLower(p.user.Name), strings.ToLower(other.user.Name)),
	)
}

type userSolve struct {
	progress *userProgress
	time     time.Time
	puzzle   string
	part     int
}

func (p *userSolve) Compare(other *userSolve) int {
	return p.time.Compare(other.time)
}

type pointKey struct {
	puzzle string
	part   int
}

func prepareSolves(a *auth.Auth, event puzzles.Event) ([]*userProgress, []*userSolve, map[pointKey]int) {
	progress, err := a.AllProgress(event.ID)
	if err != nil {
		panic(fmt.Errorf("get all progress: %w", err))
	}

	ups := make([]*userProgress, 0, len(progress))
	for user, p := range progress {
		if user.Hidden {
			continue
		}

		ups = append(ups, &userProgress{
			user:    user,
			puzzles: p.Puzzles,
		})
	}

	partsTotal := 0
	for _, puzzle := range event.Puzzles {
		partsTotal += len(puzzle.Parts)
	}

	solves := make([]*userSolve, 0, len(ups)*partsTotal)
	for _, up := range ups {
		for puzzle, prog := range up.puzzles {
			for i, part := range prog.Parts {
				solves = append(solves, &userSolve{
					progress: up,
					time:     part.Time,
					puzzle:   puzzle,
					part:     i,
				})
			}
		}
	}
	slices.SortFunc(solves, (*userSolve).Compare)

	points := make(map[pointKey]int, partsTotal)
	for _, puzzle := range event.Puzzles {
		for i := range puzzle.Parts {
			points[pointKey{
				puzzle: puzzle.ID,
				part:   i,
			}] = len(ups)
		}
	}

	return ups, solves, points
}

func leaderboardMiddleware(a *auth.Auth, event puzzles.Event, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pd := pageDataFromContext(r.Context())

		chartNow := nowForChart(pd.Now)
		chartAt := chartNow
		if pd.User != nil && pd.User.Admin {
			chartMin := chartNow
			chartMax := chartNow
			if len(event.Puzzles) > 0 {
				chartMin = event.Puzzles[0].Unlock
				for _, puzzle := range event.Puzzles {
					if puzzle.Unlock.Before(chartMin) {
						chartMin = puzzle.Unlock
					}
				}
				chartMax = event.Puzzles[len(event.Puzzles)-1].Unlock.Add(48 * time.Hour)
			}

			if chartAt.Before(chartMin) {
				chartAt = chartMin
			}
			if chartAt.After(chartMax) {
				chartAt = chartMax
			}

			pd.LeaderboardChartAtMin = chartMin.Unix()
			pd.LeaderboardChartAtMax = chartMax.Unix()
			pd.LeaderboardChartAt = chartAt.Unix()
		}

		pd.LeaderboardChart = template.HTML(buildLeaderboardChart(chartAt, a, event))

		ups, solves, points := prepareSolves(a, event)

		for _, solve := range solves {
			up := solve.progress

			up.solved++

			pk := pointKey{
				puzzle: solve.puzzle,
				part:   solve.part,
			}
			up.score += points[pk]
			points[pk]--

			if solve.time.After(up.lastSolve) {
				up.lastSolve = solve.time
			}
		}

		slices.SortFunc(ups, (*userProgress).Compare)
		if len(ups) > 50 {
			ups = ups[:50]
		}

		for _, up := range ups {
			pd.Leaderboard = append(pd.Leaderboard, leaderboardData{
				User: userData{
					ID:     up.user.ID,
					Name:   up.user.Name,
					Avatar: up.user.AvatarURL,
				},
				Solved: up.solved,
				Score:  up.score,
			})
		}

		next.ServeHTTP(w, r)
	})
}

func nowForChart(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now()
	}
	return now
}

func leaderboardAxisEndTime(event puzzles.Event, now time.Time) time.Time {
	end := now

	if len(event.Puzzles) > 0 {
		firstUnlock := event.Puzzles[0].Unlock
		lastUnlock := event.Puzzles[len(event.Puzzles)-1].Unlock

		if end.Before(firstUnlock) {
			end = firstUnlock
		}

		maxEnd := lastUnlock.Add(48 * time.Hour)
		if end.After(maxEnd) {
			end = maxEnd
		}
	}

	return end
}

func filterSolvesAtOrBefore(solves []*userSolve, now time.Time) []*userSolve {
	visible := solves[:0]
	for _, solve := range solves {
		if solve.time.After(now) {
			continue
		}
		visible = append(visible, solve)
	}
	return visible
}

func buildLeaderboardChart(now time.Time, a *auth.Auth, event puzzles.Event) []byte {
	ups, solves, points := prepareSolves(a, event)
	solves = filterSolvesAtOrBefore(solves, now)

	const (
		width   = 880
		left    = 18.0
		right   = 180.0
		top     = 28.0
		bottom  = 70.0
		maxRank = 50
		textCol = "#5865f2"
		gridCol = "#808080"
		rowGap  = 22.0
	)

	plotW := float64(width) - left - right

	if len(ups) == 0 {
		height := 420
		return fmt.Appendf(nil, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\"><text x=\"50%%\" y=\"50%%\" text-anchor=\"middle\" fill=\"%s\" font-size=\"24\" font-family=\"monospace\" font-weight=\"700\">No leaderboard data yet</text></svg>", width, height, width, height, textCol)
	}

	rankLimit := min(maxRank, len(ups))
	if rankLimit <= 0 {
		rankLimit = 1
	}

	rows := max(rankLimit-1, 1)
	plotH := float64(rows) * rowGap
	height := int(top + plotH + bottom)

	type unlockMark struct {
		name   string
		unlock time.Time
	}
	unlockMarks := make([]unlockMark, 0, len(event.Puzzles))

	start := event.Puzzles[0].Unlock
	end := leaderboardAxisEndTime(event, now)
	for _, puzzle := range event.Puzzles {
		unlockMarks = append(unlockMarks, unlockMark{
			name:   puzzle.Name,
			unlock: puzzle.Unlock,
		})

		if puzzle.Unlock.Before(start) {
			start = puzzle.Unlock
		}
	}

	slices.SortFunc(unlockMarks, func(a, b unlockMark) int {
		return a.unlock.Compare(b.unlock)
	})

	if end.Before(start) {
		end = start
	}
	totalRange := end.Sub(start)
	if totalRange <= 0 {
		totalRange = time.Second
	}

	type chartPoint struct {
		idx   int
		time  time.Time
		rank  int
		solve bool
	}

	type chartSeries struct {
		name   string
		userID string
		points []chartPoint
	}

	series := make(map[string]*chartSeries, len(ups))
	for _, up := range ups {
		series[up.user.ID] = &chartSeries{
			name:   up.user.Name,
			userID: up.user.ID,
			points: make([]chartPoint, 0, 16),
		}
	}

	for i, solve := range solves {
		up := solve.progress

		up.solved++

		pk := pointKey{
			puzzle: solve.puzzle,
			part:   solve.part,
		}
		up.score += points[pk]
		points[pk]--

		if solve.time.After(up.lastSolve) {
			up.lastSolve = solve.time
		}

		slices.SortFunc(ups, (*userProgress).Compare)

		topN := min(maxRank, len(ups))
		for rank := 1; rank <= topN; rank++ {
			cur := ups[rank-1]
			s := series[cur.user.ID]
			s.points = append(s.points, chartPoint{
				idx:   i,
				time:  solve.time,
				rank:  rank,
				solve: cur.user.ID == up.user.ID,
			})
		}
	}

	timeX := func(t time.Time) float64 {
		ratio := float64(t.Sub(start)) / float64(totalRange)
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		return left + ratio*plotW
	}

	type solveGroup struct {
		x     float64
		count int
	}

	rawSolveX := make([]float64, len(solves))
	for i, solve := range solves {
		rawSolveX[i] = timeX(solve.time)
	}

	keepSolveIdx := make([]bool, len(solves))
	solveToClump := make([]int, len(solves))
	for i := range solveToClump {
		solveToClump[i] = -1
	}
	solveGroups := make([]solveGroup, 0, len(solves))

	// Fixed clump distance in pixels to keep behavior predictable.
	clumpGap := 8.0

	for i := 0; i < len(rawSolveX); {
		j := i + 1
		for j < len(rawSolveX) {
			if rawSolveX[j]-rawSolveX[j-1] > clumpGap {
				break
			}
			j++
		}

		clumpIdx := len(solveGroups)
		for k := i; k < j; k++ {
			solveToClump[k] = clumpIdx
		}

		keep := j - 1
		keepSolveIdx[keep] = true

		groupX := rawSolveX[keep]
		count := j - i
		solveGroups = append(solveGroups, solveGroup{
			x:     groupX,
			count: count,
		})
		i = j
	}

	clumpSolvedByUser := make([]map[string]bool, len(solveGroups))
	clumpLastSolveIdxByUser := make([]map[string]int, len(solveGroups))
	for i, solve := range solves {
		clumpIdx := solveToClump[i]
		if clumpIdx < 0 || clumpIdx >= len(clumpSolvedByUser) {
			continue
		}
		if clumpSolvedByUser[clumpIdx] == nil {
			clumpSolvedByUser[clumpIdx] = make(map[string]bool)
		}
		userID := solve.progress.user.ID
		clumpSolvedByUser[clumpIdx][userID] = true
		if clumpLastSolveIdxByUser[clumpIdx] == nil {
			clumpLastSolveIdxByUser[clumpIdx] = make(map[string]int)
		}
		clumpLastSolveIdxByUser[clumpIdx][userID] = i
	}

	pointX := func(idx int) float64 {
		if idx >= 0 && idx < len(rawSolveX) {
			return rawSolveX[idx]
		}
		return left
	}

	rankY := func(rank int) float64 {
		if rankLimit <= 1 {
			return top
		}
		ratio := float64(rank-1) / float64(rankLimit-1)
		return top + ratio*plotH
	}

	var b strings.Builder
	b.Grow(64 * 1024)
	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\" font-weight=\"700\">", width, height, width, height)

	for _, grp := range solveGroups {
		opacity := 0.16 + 0.02*float64(grp.count-1)
		if opacity > 0.52 {
			opacity = 0.52
		}
		strokeW := 1.0
		if grp.count > 1 {
			strokeW = 1.2
		}
		fmt.Fprintf(&b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-opacity=\"%.2f\" stroke-width=\"%.1f\"/>", grp.x, top, grp.x, top+plotH, gridCol, opacity, strokeW)
	}

	for rank := 1; rank <= rankLimit; rank++ {
		y := rankY(rank)
		strokeOpacity := "0.22"
		if rank == 1 || rank == rankLimit || rank == 10 || rank == 25 {
			strokeOpacity = "0.36"
		}
		fmt.Fprintf(&b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-opacity=\"%s\" stroke-width=\"1\"/>", left, y, left+plotW, y, gridCol, strokeOpacity)
	}

	guideX := left + plotW
	fmt.Fprintf(&b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-opacity=\"0.45\" stroke-width=\"2\" stroke-dasharray=\"4 4\"/>", guideX, top, guideX, top+plotH, gridCol)

	for _, mark := range unlockMarks {
		if mark.unlock.Before(start) || mark.unlock.After(end) {
			continue
		}

		x := timeX(mark.unlock)
		fmt.Fprintf(&b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-opacity=\"0.8\" stroke-width=\"2.2\"/>", x, top, x, top+plotH, textCol)

		label := html.EscapeString(mark.name)
		y := top + plotH + 14
		fmt.Fprintf(&b, "<text x=\"%.2f\" y=\"%.2f\" text-anchor=\"start\" transform=\"rotate(15 %.2f %.2f)\" fill=\"%s\" font-size=\"13\" font-family=\"monospace\">%s</text>", x+2, y, x+2, y, textCol, label)
	}

	fmt.Fprintf(&b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-opacity=\"0.7\" stroke-width=\"1.5\"/>", left, top, left, top+plotH, gridCol)
	fmt.Fprintf(&b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-opacity=\"0.7\" stroke-width=\"1.5\"/>", left, top+plotH, left+plotW, top+plotH, gridCol)

	rankLabels := []int{1}
	for _, rank := range []int{10, 25, 50} {
		if rank > 1 && rank < rankLimit {
			rankLabels = append(rankLabels, rank)
		}
	}
	if rankLimit > 1 {
		rankLabels = append(rankLabels, rankLimit)
	}
	slices.Sort(rankLabels)
	rankLabels = slices.Compact(rankLabels)
	for _, rank := range rankLabels {
		y := rankY(rank)
		fmt.Fprintf(&b, "<text x=\"%.2f\" y=\"%.2f\" text-anchor=\"end\" dominant-baseline=\"middle\" fill=\"%s\" font-size=\"13\" font-family=\"monospace\">%d</text>", left-3, y, textCol, rank)
	}

	seriesList := make([]*chartSeries, 0, len(series))
	for _, s := range series {
		if len(s.points) == 0 {
			continue
		}
		seriesList = append(seriesList, s)
	}
	slices.SortFunc(seriesList, func(a, b *chartSeries) int {
		la := a.points[len(a.points)-1].rank
		lb := b.points[len(b.points)-1].rank
		return cmp.Or(cmp.Compare(la, lb), cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name)))
	})

	lineColors := make(map[string]string, len(seriesList))
	baseHue := 212.0
	for i, s := range seriesList {
		hue := math.Mod(baseHue+float64(i)*137.507764, 360)
		sat := 74
		light := 44
		switch i % 3 {
		case 1:
			sat = 82
			light = 40
		case 2:
			sat = 68
			light = 50
		}
		lineColors[s.userID] = fmt.Sprintf("hsl(%.1f %d%% %d%%)", hue, sat, light)
	}

	for _, s := range seriesList {
		color := lineColors[s.userID]

		playerSolvedInClump := func(solveIdx int) bool {
			if solveIdx < 0 || solveIdx >= len(solveToClump) {
				return false
			}
			clumpIdx := solveToClump[solveIdx]
			if clumpIdx < 0 || clumpIdx >= len(clumpSolvedByUser) {
				return false
			}
			return clumpSolvedByUser[clumpIdx][s.userID]
		}

		playerPointX := func(solveIdx int) float64 {
			if solveIdx < 0 || solveIdx >= len(solveToClump) {
				return pointX(solveIdx)
			}

			clumpIdx := solveToClump[solveIdx]
			if clumpIdx < 0 || clumpIdx >= len(clumpLastSolveIdxByUser) {
				return pointX(solveIdx)
			}

			if lastSolveIdx, ok := clumpLastSolveIdxByUser[clumpIdx][s.userID]; ok {
				return pointX(lastSolveIdx)
			}

			return pointX(solveIdx)
		}

		displayPoints := make([]chartPoint, 0, len(s.points))
		for _, p := range s.points {
			if keepSolveIdx[p.idx] {
				displayPoints = append(displayPoints, p)
			}
		}
		if len(displayPoints) == 0 {
			continue
		}

		firstSolveIdx := -1
		for i, p := range displayPoints {
			if playerSolvedInClump(p.idx) {
				firstSolveIdx = i
				break
			}
		}
		if firstSolveIdx < 0 {
			continue
		}

		startIdx := firstSolveIdx
		for startIdx < len(displayPoints) {
			endIdx := startIdx
			for endIdx+1 < len(displayPoints) && solveToClump[displayPoints[endIdx+1].idx] == solveToClump[displayPoints[endIdx].idx]+1 {
				endIdx++
			}

			if endIdx > startIdx {
				b.WriteString("<polyline fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\"")
				fmt.Fprintf(&b, " stroke=\"%s\" stroke-opacity=\"0.78\" stroke-width=\"1.7\" points=\"", color)
				for i := startIdx; i <= endIdx; i++ {
					p := displayPoints[i]
					fmt.Fprintf(&b, "%.2f,%.2f ", playerPointX(p.idx), rankY(p.rank))
				}
				b.WriteString("\"/>")
			}

			startIdx = endIdx + 1
		}

		last := displayPoints[len(displayPoints)-1]
		for _, p := range displayPoints {
			if !playerSolvedInClump(p.idx) {
				continue
			}
			fmt.Fprintf(&b, "<circle cx=\"%.2f\" cy=\"%.2f\" r=\"3.4\" fill=\"%s\" stroke=\"#fff\" stroke-width=\"0.9\"/>", playerPointX(p.idx), rankY(p.rank), color)
		}

		lastX := playerPointX(last.idx)
		y := rankY(last.rank)
		rightX := left + plotW
		if lastX < rightX {
			fmt.Fprintf(&b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-opacity=\"0.78\" stroke-width=\"1.7\" stroke-linecap=\"round\"/>", lastX, y, rightX, y, color)
		}

		fmt.Fprintf(&b, "<text x=\"%.2f\" y=\"%.2f\" text-anchor=\"start\" dominant-baseline=\"middle\" fill=\"%s\" font-size=\"13\" font-family=\"monospace\">%s</text>", rightX+4, y, color, html.EscapeString(s.name))
	}

	b.WriteString("</svg>")

	return []byte(b.String())
}

func leaderboardChart(a *auth.Auth, event puzzles.Event) http.Handler {
	writeResponse := func(w http.ResponseWriter, r *http.Request, svg []byte, cacheControl string) {
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		w.Header().Set("Cache-Control", cacheControl)
		w.Header().Set("Content-Length", strconv.Itoa(len(svg)))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(svg); err != nil {
			logger := ctxlog.Get(r.Context())
			logger.Error("failed to write response", "error", err)
			return
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		cacheControl := "no-store"
		if rawAt := r.URL.Query().Get("at"); rawAt != "" {
			at, err := time.Parse(time.RFC3339, rawAt)
			if err != nil {
				http.Error(w, "invalid at query parameter; expected RFC3339 datetime", http.StatusBadRequest)
				return
			}
			now = at
			cacheControl = "public, max-age=300, stale-while-revalidate=300, stale-if-error=86400"
		}

		svg := buildLeaderboardChart(now, a, event)
		writeResponse(w, r, svg, cacheControl)
	})
}
