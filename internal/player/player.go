// Package player plays music on the machine the server runs on by driving
// an mpv process over its JSON IPC channel. This is what makes a headless
// board (Raspberry Pi, Orange Pi) a speaker you control from another device.
package player

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

// State is the player state sent to clients.
type State struct {
	Available bool     `json:"available"` // mpv was found
	Queue     []string `json:"queue"`
	Index     int      `json:"index"`
	Playing   bool     `json:"playing"` // a track is loaded
	Paused    bool     `json:"paused"`
	Position  float64  `json:"position"`
	Duration  float64  `json:"duration"`
	Volume    float64  `json:"volume"`
	Repeat    string   `json:"repeat"` // off | all | one
	Error     string   `json:"error,omitempty"`
}

// AudioDevice is an output mpv can play to.
type AudioDevice struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type response struct {
	Error string          `json:"error"`
	Data  json.RawMessage `json:"data"`
}

type Player struct {
	mpvPath string
	// Resolve maps a track ID to something mpv can open.
	Resolve    func(id string) (string, bool)
	OnChange   func()
	OnPlay     func(id string)
	OnDuration func(id string, seconds float64)

	opMu sync.Mutex // serialises user-facing operations

	mu       sync.Mutex // guards everything below
	st       State
	conn     io.ReadWriteCloser
	cmd      *exec.Cmd
	pending  map[int]chan response
	nextReq  int
	lastSec  int
	failures int
	device   string
}

// New returns a player. mpvPath may be empty to look mpv up in PATH.
func New(mpvPath string) *Player {
	if mpvPath == "" {
		mpvPath, _ = exec.LookPath("mpv")
	}
	return &Player{
		mpvPath: mpvPath,
		pending: map[int]chan response{},
		st:      State{Available: mpvPath != "", Queue: []string{}, Volume: 80, Repeat: "off", Index: -1},
	}
}

func (p *Player) Available() bool { return p.mpvPath != "" }

func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	st := p.st
	st.Queue = append([]string{}, p.st.Queue...)
	return st
}

func (p *Player) changed() {
	if p.OnChange != nil {
		p.OnChange()
	}
}

// ensure starts mpv if it is not running.
func (p *Player) ensure() error {
	if p.mpvPath == "" {
		return errors.New("mpv is not installed on the server")
	}
	p.mu.Lock()
	running := p.conn != nil
	volume, device := p.st.Volume, p.device
	p.mu.Unlock()
	if running {
		return nil
	}

	path := ipcPath()
	cmd := exec.Command(p.mpvPath, "--idle=yes", "--no-video", "--no-terminal",
		"--audio-display=no", "--input-ipc-server="+path)
	if err := cmd.Start(); err != nil {
		return err
	}
	var conn io.ReadWriteCloser
	var err error
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if conn, err = dialIPC(path); err == nil {
			break
		}
	}
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return errors.New("could not connect to mpv: " + err.Error())
	}

	p.mu.Lock()
	p.conn, p.cmd = conn, cmd
	p.mu.Unlock()
	go p.readLoop(conn, cmd)

	for i, name := range []string{"time-pos", "duration", "pause", "volume"} {
		p.command("observe_property", i+1, name)
	}
	p.command("set_property", "volume", volume)
	if device != "" {
		p.command("set_property", "audio-device", device)
	}
	return nil
}

func (p *Player) command(args ...any) (json.RawMessage, error) {
	p.mu.Lock()
	conn := p.conn
	if conn == nil {
		p.mu.Unlock()
		return nil, errors.New("mpv is not running")
	}
	p.nextReq++
	id := p.nextReq
	ch := make(chan response, 1)
	p.pending[id] = ch
	msg, _ := json.Marshal(map[string]any{"command": args, "request_id": id})
	_, err := conn.Write(append(msg, '\n'))
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case r, ok := <-ch:
		if !ok {
			return nil, errors.New("mpv exited")
		}
		if r.Error != "success" {
			return nil, errors.New("mpv: " + r.Error)
		}
		return r.Data, nil
	case <-time.After(5 * time.Second):
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, errors.New("mpv did not respond")
	}
}

func (p *Player) readLoop(conn io.ReadWriteCloser, cmd *exec.Cmd) {
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var m struct {
			Event     string          `json:"event"`
			Name      string          `json:"name"`
			Reason    string          `json:"reason"`
			RequestID int             `json:"request_id"`
			Error     string          `json:"error"`
			Data      json.RawMessage `json:"data"`
		}
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		switch m.Event {
		case "":
			p.mu.Lock()
			ch := p.pending[m.RequestID]
			delete(p.pending, m.RequestID)
			p.mu.Unlock()
			if ch != nil {
				ch <- response{Error: m.Error, Data: m.Data}
			}
		case "property-change":
			p.propertyChanged(m.Name, m.Data)
		case "end-file":
			// "stop" means we replaced or stopped the file ourselves.
			switch m.Reason {
			case "eof":
				go p.advance(true, false)
			case "error":
				go p.advance(true, true)
			}
		}
	}
	conn.Close()
	cmd.Process.Kill()
	cmd.Wait()
	p.mu.Lock()
	if p.conn == conn {
		p.conn, p.cmd = nil, nil
		p.st.Playing, p.st.Position = false, 0
	}
	for id, ch := range p.pending {
		close(ch)
		delete(p.pending, id)
	}
	p.mu.Unlock()
	p.changed()
}

func (p *Player) propertyChanged(name string, data json.RawMessage) {
	var num float64
	var flag bool
	notify := true
	var durID string
	p.mu.Lock()
	switch name {
	case "time-pos":
		if json.Unmarshal(data, &num) == nil {
			p.st.Position = num
			// Position ticks many times a second; tell clients once a second.
			if sec := int(num); sec == p.lastSec {
				notify = false
			} else {
				p.lastSec = sec
			}
		} else {
			notify = false
		}
	case "duration":
		if json.Unmarshal(data, &num) == nil {
			p.st.Duration = num
			if p.st.Index >= 0 && p.st.Index < len(p.st.Queue) {
				durID = p.st.Queue[p.st.Index]
			}
		}
	case "pause":
		if json.Unmarshal(data, &flag) == nil {
			p.st.Paused = flag
		}
	case "volume":
		if json.Unmarshal(data, &num) == nil {
			p.st.Volume = num
		}
	}
	p.mu.Unlock()
	if durID != "" && num > 0 && p.OnDuration != nil {
		p.OnDuration(durID, num)
	}
	if notify {
		p.changed()
	}
}

// loadCurrent starts the track at st.Index. Caller holds opMu.
func (p *Player) loadCurrent() error {
	p.mu.Lock()
	if p.st.Index < 0 || p.st.Index >= len(p.st.Queue) {
		p.mu.Unlock()
		return errors.New("nothing to play")
	}
	id := p.st.Queue[p.st.Index]
	p.mu.Unlock()

	src, ok := p.Resolve(id)
	if !ok {
		return errors.New("track not found")
	}
	if err := p.ensure(); err != nil {
		p.setError(err)
		return err
	}
	if _, err := p.command("loadfile", src, "replace"); err != nil {
		p.setError(err)
		return err
	}
	p.command("set_property", "pause", false)
	p.mu.Lock()
	p.st.Playing, p.st.Paused = true, false
	p.st.Position, p.st.Duration, p.st.Error = 0, 0, ""
	p.lastSec = -1
	p.mu.Unlock()
	if p.OnPlay != nil {
		p.OnPlay(id)
	}
	p.changed()
	return nil
}

func (p *Player) setError(err error) {
	log.Printf("player: %v", err)
	p.mu.Lock()
	p.st.Error = err.Error()
	p.st.Playing = false
	p.mu.Unlock()
	p.changed()
}

// advance moves to the next track. auto is true when a track ended by itself.
func (p *Player) advance(auto, failed bool) {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	n := len(p.st.Queue)
	if failed {
		p.failures++
	} else {
		p.failures = 0
	}
	giveUp := n == 0 || p.failures > n
	next := p.st.Index + 1
	switch {
	case auto && !failed && p.st.Repeat == "one":
		next = p.st.Index
	case next >= n && (p.st.Repeat == "all" || !auto):
		next = 0
	}
	if giveUp || next >= n {
		p.st.Playing, p.st.Position = false, 0
		p.failures = 0
		p.mu.Unlock()
		p.changed()
		return
	}
	p.st.Index = next
	p.mu.Unlock()
	p.loadCurrent()
}

// SetQueue replaces the queue. With play=false the current track keeps
// playing if it is still the one at index (used for reorder and shuffle).
func (p *Player) SetQueue(ids []string, index int, play bool) error {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	if ids == nil {
		ids = []string{}
	}
	if index < 0 || index >= len(ids) {
		index = 0
	}
	p.mu.Lock()
	p.st.Queue = ids
	p.st.Index = index
	if len(ids) == 0 {
		p.st.Index = -1
	}
	p.failures = 0
	p.mu.Unlock()
	if play && len(ids) > 0 {
		return p.loadCurrent()
	}
	p.changed()
	return nil
}

func (p *Player) Jump(index int) error {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	if index < 0 || index >= len(p.st.Queue) {
		p.mu.Unlock()
		return errors.New("index out of range")
	}
	p.st.Index = index
	p.mu.Unlock()
	return p.loadCurrent()
}

func (p *Player) Next() { p.advance(false, false) }

func (p *Player) Prev() error {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	restart := p.st.Position > 3 || p.st.Index <= 0
	if !restart {
		p.st.Index--
	}
	playing := p.st.Playing
	p.mu.Unlock()
	if restart && playing {
		_, err := p.command("seek", 0, "absolute")
		return err
	}
	return p.loadCurrent()
}

// SetPaused pauses or resumes. Resuming with nothing loaded starts the queue.
func (p *Player) SetPaused(paused bool) error {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	playing := p.st.Playing
	p.mu.Unlock()
	if !playing {
		if paused {
			return nil
		}
		return p.loadCurrent()
	}
	_, err := p.command("set_property", "pause", paused)
	if err == nil {
		p.mu.Lock()
		p.st.Paused = paused
		p.mu.Unlock()
	}
	return err
}

func (p *Player) Toggle() error {
	p.mu.Lock()
	paused := p.st.Paused || !p.st.Playing
	p.mu.Unlock()
	return p.SetPaused(!paused)
}

func (p *Player) Stop() {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.command("stop")
	p.mu.Lock()
	p.st.Playing, p.st.Position = false, 0
	p.mu.Unlock()
	p.changed()
}

func (p *Player) Seek(seconds float64) error {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	_, err := p.command("seek", seconds, "absolute")
	return err
}

func (p *Player) SetVolume(v float64) error {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	p.opMu.Lock()
	defer p.opMu.Unlock()
	p.mu.Lock()
	p.st.Volume = v
	running := p.conn != nil
	p.mu.Unlock()
	if !running {
		p.changed()
		return nil
	}
	_, err := p.command("set_property", "volume", v)
	return err
}

func (p *Player) SetRepeat(mode string) error {
	if mode != "off" && mode != "all" && mode != "one" {
		return errors.New("repeat must be off, all or one")
	}
	p.mu.Lock()
	p.st.Repeat = mode
	p.mu.Unlock()
	p.changed()
	return nil
}

// AudioDevices lists mpv's outputs and the selected one.
func (p *Player) AudioDevices() ([]AudioDevice, string, error) {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	if err := p.ensure(); err != nil {
		return nil, "", err
	}
	data, err := p.command("get_property", "audio-device-list")
	if err != nil {
		return nil, "", err
	}
	var list []AudioDevice
	json.Unmarshal(data, &list)
	current := "auto"
	if data, err := p.command("get_property", "audio-device"); err == nil {
		json.Unmarshal(data, &current)
	}
	return list, current, nil
}

func (p *Player) SetAudioDevice(name string) error {
	p.opMu.Lock()
	defer p.opMu.Unlock()
	if err := p.ensure(); err != nil {
		return err
	}
	if _, err := p.command("set_property", "audio-device", name); err != nil {
		return err
	}
	p.mu.Lock()
	p.device = name
	p.mu.Unlock()
	return nil
}

// Close stops mpv.
func (p *Player) Close() {
	p.mu.Lock()
	cmd := p.cmd
	p.mu.Unlock()
	if cmd != nil {
		p.command("quit")
		time.Sleep(100 * time.Millisecond)
		cmd.Process.Kill()
	}
}
