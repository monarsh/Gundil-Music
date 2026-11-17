package player

import (
	"testing"
	"time"

	"github.com/monarsh/gundil-music/internal/model"
)

func ids(p *Player) string {
	s := ""
	for _, t := range p.State().Queue {
		s += t.VideoID[:1]
	}
	return s
}

func queue(index int) *Player {
	p := &Player{pending: map[int]chan response{}, st: State{Repeat: "off"}}
	for _, c := range "abcde" {
		p.st.Queue = append(p.st.Queue, model.Track{VideoID: string(c) + "0000000000", Title: string(c)})
	}
	p.st.Index = index
	return p
}

func TestMove(t *testing.T) {
	cases := []struct {
		from, to, index int
		want            string
		wantIndex       int
	}{
		{0, 3, 2, "bcdae", 1}, // from before the current song to after it
		{4, 0, 2, "eabcd", 3}, // from after to before
		{2, 4, 2, "abdec", 4}, // the current song itself
		{3, 4, 1, "abced", 1}, // both after
		{1, 1, 1, "abcde", 1}, // no change
	}
	for _, c := range cases {
		p := queue(c.index)
		if err := p.Move(c.from, c.to); err != nil {
			t.Fatal(err)
		}
		if got := ids(p); got != c.want || p.State().Index != c.wantIndex {
			t.Errorf("Move(%d,%d) at %d = %s index %d, want %s index %d", c.from, c.to, c.index, got, p.State().Index, c.want, c.wantIndex)
		}
	}
	if err := queue(0).Move(0, 9); err == nil {
		t.Error("out of range move accepted")
	}
}

func TestSleepAtEnd(t *testing.T) {
	p := queue(1)
	p.st.Playing = true
	p.SetSleep(0, true)
	p.advance(true, false)
	st := p.State()
	if st.Playing || st.SleepEnd || st.Index != 2 {
		t.Fatalf("after the song ended: playing=%v sleepEnd=%v index=%d", st.Playing, st.SleepEnd, st.Index)
	}
}

func TestSleepTimer(t *testing.T) {
	p := queue(0)
	p.SetSleep(time.Hour, false)
	if p.State().SleepAt == 0 {
		t.Fatal("timer not set")
	}
	p.SetSleep(0, false)
	if st := p.State(); st.SleepAt != 0 || st.SleepEnd || p.sleep != nil {
		t.Fatal("timer not cleared")
	}
}
