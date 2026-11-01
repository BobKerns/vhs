package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/vhs/lexer"
	"github.com/charmbracelet/vhs/parser"
)

func TestFrameClockOnTimeIsNotBehind(t *testing.T) {
	start := time.Unix(0, 0)
	interval := 20 * time.Millisecond
	c := newFrameClock(interval, start)
	for i := 1; i <= 50; i++ {
		c.tick(start.Add(time.Duration(i)*interval), true)
		c.frame()
	}
	r := c.report(50)
	if r.Frames != 50 || r.Seconds != 1 || r.Behind != 0 {
		t.Fatalf("got %+v", r)
	}
}

func TestFrameClockCountsLateFramesAndSkipsHiddenTime(t *testing.T) {
	start := time.Unix(0, 0)
	interval := 20 * time.Millisecond
	c := newFrameClock(interval, start)
	now := start
	step := func(d time.Duration, recording, written bool) {
		now = now.Add(d)
		c.tick(now, recording)
		if written {
			c.frame()
		}
	}
	for range 10 { // hidden: ticks, no frames, no time
		step(interval, false, false)
	}
	for range 10 {
		step(interval, true, true)
	}
	step(220*time.Millisecond, true, true) // one frame 200 ms late
	step(interval, true, false)            // a failed capture: time passes, no frame
	r := c.report(50)
	if r.Frames != 11 {
		t.Fatalf("frames %d", r.Frames)
	}
	if math.Abs(r.Seconds-0.44) > 1e-9 || math.Abs(r.Behind-0.22) > 1e-9 || math.Abs(r.WorstGap-0.22) > 1e-9 {
		t.Fatalf("got %+v", r)
	}
}

func TestWriteTimingOnlyWhenAsked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "timing.json")
	t.Setenv(timingFileEnv, "")
	if err := writeTiming(timingReport{Frames: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("wrote a report nobody asked for")
	}
	t.Setenv(timingFileEnv, path)
	if err := writeTiming(timingReport{Framerate: 50, Frames: 3, Seconds: 0.1, Behind: 0.04}); err != nil {
		t.Fatal(err)
	}
	var got map[string]float64
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["frames"] != 3 || got["behind"] != 0.04 || got["framerate"] != 50 {
		t.Fatalf("got %v", got)
	}
}

func TestFrameClockMissedSlotsWhileHeld(t *testing.T) {
	start := time.Unix(0, 0)
	interval := 20 * time.Millisecond
	c := newFrameClock(interval, start)
	c.tick(start.Add(interval), true)
	if c.missed() != 1 {
		t.Fatalf("on time, only its own slot is due: %d", c.missed())
	}
	c.frame()
	c.tick(start.Add(5*interval), true) // three slots late
	held := 0
	for c.missed() > 1 {
		c.hold()
		held++
	}
	c.frame()
	r := c.report(50)
	if held != 3 || r.Held != 3 || r.Frames != 5 || r.Behind != 0 {
		t.Fatalf("held %d, %+v", held, r)
	}
	if math.Abs(r.WorstGap-0.08) > 1e-9 {
		t.Fatalf("worst gap %v", r.WorstGap)
	}
}

func TestRealTimeSetting(t *testing.T) {
	v := VHS{Options: &Options{}}
	if v.Options.Video.RealTime {
		t.Fatal("real time must default to off (upstream's behavior)")
	}
	for _, value := range []string{"true", "false"} {
		p := parser.New(lexer.New("Set RealTime " + value))
		commands := p.Parse()
		if len(p.Errors()) != 0 || len(commands) != 1 {
			t.Fatalf("parse %s: %v", value, p.Errors())
		}
		requireNoErr(t, Execute(commands[0], &v))
		if v.Options.Video.RealTime != (value == "true") {
			t.Fatalf("setting %s not applied", value)
		}
	}
	p := parser.New(lexer.New("Set RealTime yes"))
	p.Parse()
	if len(p.Errors()) == 0 {
		t.Fatal("accepted an invalid boolean")
	}
}
