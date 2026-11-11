// Package queue is the play queue: a list of songs, the current position,
// and shuffle/repeat behaviour. It is pure logic (no I/O, no UI) so it can be
// tested without mocks.
//
// All public indices are positions in *play order*: the order songs will be
// played in. With shuffle off that is the order they were added; with shuffle
// on it is a random permutation (the current song stays first-in-line).
package queue

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

// Repeat is the repeat mode. Go has no enums; a named int type with iota
// constants is the idiom, and methods can hang off it like any other type.
type Repeat int

const (
	RepeatOff Repeat = iota
	RepeatAll
	RepeatOne
)

func (r Repeat) String() string {
	switch r {
	case RepeatAll:
		return "all"
	case RepeatOne:
		return "one"
	}
	return "off"
}

// Queue holds songs and a play position. The zero value is not usable; call
// New. Methods that change state use pointer receivers, so callers share one
// Queue instead of copying it.
type Queue struct {
	items []ytmusic.Song // songs in insertion order
	order []int          // play order: order[i] is an index into items
	pos   int            // index into order; -1 = nothing played yet

	// detached means the current song was removed. pos then points at the
	// song *before* the gap, so Next and Prev still do the natural thing.
	detached bool

	shuffle bool
	repeat  Repeat
	rng     *rand.Rand
}

// New returns an empty queue. rng drives shuffling; pass a seeded generator
// in tests for deterministic results, or nil for a time-seeded one.
func New(rng *rand.Rand) *Queue {
	if rng == nil {
		rng = rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0))
	}
	return &Queue{pos: -1, rng: rng}
}

func (q *Queue) Len() int              { return len(q.order) }
func (q *Queue) Shuffle() bool         { return q.shuffle }
func (q *Queue) Repeat() Repeat        { return q.repeat }
func (q *Queue) Position() int         { return q.curPos() }
func (q *Queue) at(i int) ytmusic.Song { return q.items[q.order[i]] }

// curPos is the play-order position of the current song, or -1 if there is
// none (nothing played yet, or it was removed).
func (q *Queue) curPos() int {
	if q.detached || q.pos < 0 || q.pos >= len(q.order) {
		return -1
	}
	return q.pos
}

// Songs returns a copy of the queue in play order. Returning a copy keeps
// callers from mutating our internal slice.
func (q *Queue) Songs() []ytmusic.Song {
	out := make([]ytmusic.Song, len(q.order))
	for i := range q.order {
		out[i] = q.at(i)
	}
	return out
}

// Current returns the song at the current position.
func (q *Queue) Current() (ytmusic.Song, bool) {
	if p := q.curPos(); p >= 0 {
		return q.at(p), true
	}
	return ytmusic.Song{}, false
}

// Replace swaps the whole queue for songs and makes songs[start] current.
func (q *Queue) Replace(songs []ytmusic.Song, start int) (ytmusic.Song, bool) {
	q.items = slices.Clone(songs)
	q.order = identity(len(songs))
	q.pos, q.detached = -1, false
	if start < 0 || start >= len(songs) {
		return ytmusic.Song{}, false
	}
	q.pos = start
	if q.shuffle {
		q.shuffleKeepingCurrent()
	}
	return q.at(q.pos), true
}

// Add appends songs. When shuffled, each one lands at a random spot among the
// songs that have not played yet.
func (q *Queue) Add(songs ...ytmusic.Song) {
	for _, s := range songs {
		q.items = append(q.items, s)
		idx := len(q.items) - 1
		if !q.shuffle {
			q.order = append(q.order, idx)
			continue
		}
		lo := q.pos + 1 // first not-yet-played slot (0 when pos is -1)
		at := lo + q.rng.IntN(len(q.order)-lo+1)
		q.order = slices.Insert(q.order, at, idx)
	}
}

// Jump makes the song at play-order position i current.
func (q *Queue) Jump(i int) (ytmusic.Song, bool) {
	if i < 0 || i >= len(q.order) {
		return ytmusic.Song{}, false
	}
	q.pos, q.detached = i, false
	return q.at(i), true
}

// Next advances. auto is true when the song ended by itself: repeat-one then
// replays it, whereas a manual skip always moves on. At the end of the queue
// it wraps with RepeatAll, otherwise reports false and stays put.
func (q *Queue) Next(auto bool) (ytmusic.Song, bool) {
	if len(q.order) == 0 {
		return ytmusic.Song{}, false
	}
	if auto && q.repeat == RepeatOne {
		if cur, ok := q.Current(); ok {
			return cur, true
		}
	}
	n := q.pos + 1 // also right when detached: pos is the song before the gap
	if n >= len(q.order) {
		if q.repeat == RepeatOff {
			return ytmusic.Song{}, false
		}
		n = 0
	}
	q.pos, q.detached = n, false
	return q.at(n), true
}

// Prev steps back. At the first song it wraps with RepeatAll, otherwise it
// returns the current song again (so the caller can restart it).
func (q *Queue) Prev() (ytmusic.Song, bool) {
	if len(q.order) == 0 {
		return ytmusic.Song{}, false
	}
	p := q.pos - 1
	if q.detached {
		p = q.pos
	}
	if p < 0 {
		if q.repeat == RepeatAll {
			p = len(q.order) - 1
		} else {
			return q.Current()
		}
	}
	q.pos, q.detached = p, false
	return q.at(p), true
}

// Remove deletes the song at play-order position i. Removing the current
// song leaves the queue "detached" (see the field comment).
func (q *Queue) Remove(i int) {
	if i < 0 || i >= len(q.order) {
		return
	}
	idx := q.order[i]
	q.items = slices.Delete(q.items, idx, idx+1)
	q.order = slices.Delete(q.order, i, i+1)
	for j, o := range q.order {
		if o > idx {
			q.order[j] = o - 1
		}
	}
	if i <= q.pos {
		if i == q.pos && !q.detached {
			q.detached = true
		}
		q.pos--
	}
}

// ToggleShuffle flips shuffle and returns the new state. The current song
// keeps playing and becomes position 0 when shuffling; when un-shuffling it
// returns to its original place.
func (q *Queue) ToggleShuffle() bool {
	q.shuffle = !q.shuffle
	cur := q.curPos()
	if q.shuffle {
		q.shuffleKeepingCurrent()
		return true
	}
	if cur >= 0 {
		q.pos = q.order[cur]
	} else {
		q.pos, q.detached = -1, false
	}
	q.order = identity(len(q.items))
	return false
}

// CycleRepeat moves off → all → one → off and returns the new mode.
func (q *Queue) CycleRepeat() Repeat {
	q.repeat = (q.repeat + 1) % 3
	return q.repeat
}

// shuffleKeepingCurrent permutes order, putting the current song (if any)
// first so it keeps playing.
func (q *Queue) shuffleKeepingCurrent() {
	cur := q.curPos()
	curIdx := -1
	if cur >= 0 {
		curIdx = q.order[cur]
	}
	rest := make([]int, 0, len(q.items))
	for i := range q.items {
		if i != curIdx {
			rest = append(rest, i)
		}
	}
	q.rng.Shuffle(len(rest), func(a, b int) { rest[a], rest[b] = rest[b], rest[a] })
	if curIdx >= 0 {
		q.order = append([]int{curIdx}, rest...)
		q.pos = 0
	} else {
		q.order = rest
		q.pos, q.detached = -1, false
	}
}

func identity(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}
