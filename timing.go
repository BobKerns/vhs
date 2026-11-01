package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// timingFileEnv names a file VHS writes its frame timing to, as JSON, when
// a recording ends. VHS numbers frames in sequence, so a frame captured late
// still plays one frame interval after the last: when capture falls behind
// (a busy machine), the video plays faster than the session it recorded.
// The report says by how much ("behind"), or, with Set RealTime, how many
// frames were held to keep time and the longest wait for one ("worst_gap",
// how long the picture froze), so a caller can record again.
const timingFileEnv = "VHS_TIMING_FILE"

// frameClock measures how a recording keeps up with real time.
type frameClock struct {
	interval time.Duration
	last     time.Time     // the previous tick, recording or not
	frames   int           // frames written, held ones included
	held     int           // frames written again because the next was late
	seconds  time.Duration // real time spent recording
	worst    time.Duration // the longest time between a frame and the tick before it
}

func newFrameClock(interval time.Duration, start time.Time) *frameClock {
	return &frameClock{interval: interval, last: start}
}

// tick notes a tick of the recorder at now, while recording or not.
func (c *frameClock) tick(now time.Time, recording bool) {
	gap := now.Sub(c.last)
	c.last = now
	if !recording {
		return
	}
	c.seconds += gap
	if gap > c.worst {
		c.worst = gap
	}
}

// frame notes that a tick's frame was written.
func (c *frameClock) frame() { c.frames++ }

// hold notes that the last frame was written again, for a slot the next one missed.
func (c *frameClock) hold() {
	c.frames++
	c.held++
}

// missed is how many frame slots are due by the last tick and not yet
// written, the slot for that tick's own frame included.
func (c *frameClock) missed() int {
	due := int((c.seconds + c.interval/2) / c.interval)
	return due - c.frames
}

// timingReport is what VHS_TIMING_FILE holds.
type timingReport struct {
	Framerate int     `json:"framerate"`
	Frames    int     `json:"frames"`
	Held      int     `json:"held"`      // frames repeated to keep real time (Set RealTime)
	Seconds   float64 `json:"seconds"`   // real time spent recording
	Behind    float64 `json:"behind"`    // how much less than that the frames play for
	WorstGap  float64 `json:"worst_gap"` // the longest wait for one frame, in seconds
}

func (c *frameClock) report(framerate int) timingReport {
	plays := time.Duration(c.frames) * c.interval
	behind := c.seconds - plays
	if behind < 0 {
		behind = 0
	}
	return timingReport{
		Framerate: framerate,
		Frames:    c.frames,
		Held:      c.held,
		Seconds:   c.seconds.Seconds(),
		Behind:    behind.Seconds(),
		WorstGap:  c.worst.Seconds(),
	}
}

// writeTiming writes the report to $VHS_TIMING_FILE, if it's set.
func writeTiming(report timingReport) error {
	path := os.Getenv(timingFileEnv)
	if path == "" {
		return nil
	}
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("error writing %s: %w", timingFileEnv, err)
	}
	return nil
}
