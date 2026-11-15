// Package bluetooth pairs and connects Bluetooth speakers and headphones on
// Linux by driving BlueZ's bluetoothctl. On other systems it reports itself
// unavailable and the operating system's own Bluetooth settings are used.
package bluetooth

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Device struct {
	MAC       string `json:"mac"`
	Name      string `json:"name"`
	Icon      string `json:"icon,omitempty"`
	Paired    bool   `json:"paired"`
	Trusted   bool   `json:"trusted"`
	Connected bool   `json:"connected"`
	Audio     bool   `json:"audio"`
}

type Status struct {
	Available bool     `json:"available"`
	Reason    string   `json:"reason,omitempty"`
	Powered   bool     `json:"powered"`
	Scanning  bool     `json:"scanning"`
	Devices   []Device `json:"devices"`
}

type Manager struct {
	bin string

	mu       sync.Mutex
	scanning bool
}

var macRe = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)

func New() *Manager {
	m := &Manager{}
	if runtime.GOOS == "linux" {
		m.bin, _ = exec.LookPath("bluetoothctl")
	}
	return m
}

func (m *Manager) Available() bool { return m.bin != "" }

func (m *Manager) run(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, m.bin, args...).CombinedOutput()
	return string(out), err
}

func (m *Manager) unavailable() Status {
	reason := "Bluetooth is managed by this system's own settings. Connect a speaker there and audio follows it."
	if runtime.GOOS == "linux" {
		reason = "bluetoothctl was not found. Install the bluez package on the server."
	}
	return Status{Reason: reason, Devices: []Device{}}
}

// Status lists the adapter state and every device BlueZ knows about.
func (m *Manager) Status() Status {
	if m.bin == "" {
		return m.unavailable()
	}
	st := Status{Available: true, Devices: []Device{}}
	m.mu.Lock()
	st.Scanning = m.scanning
	m.mu.Unlock()

	show, err := m.run(5*time.Second, "show")
	if err != nil || strings.Contains(show, "No default controller") {
		st.Available = false
		st.Reason = "No Bluetooth adapter was found on the server."
		return st
	}
	st.Powered = strings.Contains(show, "Powered: yes")

	list, _ := m.run(5*time.Second, "devices")
	for _, line := range strings.Split(list, "\n") {
		f := strings.SplitN(strings.TrimSpace(line), " ", 3)
		if len(f) < 2 || f[0] != "Device" || !macRe.MatchString(f[1]) {
			continue
		}
		d := Device{MAC: f[1], Name: f[1]}
		if len(f) == 3 {
			d.Name = f[2]
		}
		info, _ := m.run(5*time.Second, "info", d.MAC)
		for _, l := range strings.Split(info, "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(l), ": ")
			if !ok {
				continue
			}
			switch k {
			case "Alias":
				d.Name = v
			case "Icon":
				d.Icon = v
			case "Paired":
				d.Paired = v == "yes"
			case "Trusted":
				d.Trusted = v == "yes"
			case "Connected":
				d.Connected = v == "yes"
			case "UUID":
				if strings.HasPrefix(v, "Audio Sink") || strings.HasPrefix(v, "Headset") || strings.HasPrefix(v, "Handsfree") {
					d.Audio = true
				}
			}
		}
		if strings.HasPrefix(d.Icon, "audio") {
			d.Audio = true
		}
		st.Devices = append(st.Devices, d)
	}
	rank := func(d Device) int {
		switch {
		case d.Connected:
			return 0
		case d.Paired:
			return 1
		case d.Audio:
			return 2
		}
		return 3
	}
	sort.SliceStable(st.Devices, func(i, j int) bool {
		if a, b := rank(st.Devices[i]), rank(st.Devices[j]); a != b {
			return a < b
		}
		return strings.ToLower(st.Devices[i].Name) < strings.ToLower(st.Devices[j].Name)
	})
	return st
}

// Scan looks for nearby devices for the given duration, in the background.
func (m *Manager) Scan(d time.Duration, done func()) error {
	if m.bin == "" {
		return errors.New("bluetooth is not available on the server")
	}
	m.mu.Lock()
	if m.scanning {
		m.mu.Unlock()
		return nil
	}
	m.scanning = true
	m.mu.Unlock()
	go func() {
		m.run(5*time.Second, "power", "on")
		secs := int(d.Seconds())
		m.run(d+5*time.Second, "--timeout", strconv.Itoa(secs), "scan", "on")
		m.mu.Lock()
		m.scanning = false
		m.mu.Unlock()
		if done != nil {
			done()
		}
	}()
	return nil
}

func (m *Manager) check(mac string) error {
	if m.bin == "" {
		return errors.New("bluetooth is not available on the server")
	}
	if !macRe.MatchString(mac) {
		return errors.New("invalid device address")
	}
	return nil
}

func failure(action, out string) error {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		last = "no response from bluetoothctl"
	}
	return errors.New(action + " failed: " + last)
}

// Connect pairs (if needed), trusts and connects a device. Trusting lets the
// device reconnect by itself after a reboot, which matters on headless boards.
func (m *Manager) Connect(mac string) error {
	if err := m.check(mac); err != nil {
		return err
	}
	m.run(5*time.Second, "power", "on")
	info, _ := m.run(5*time.Second, "info", mac)
	if !strings.Contains(info, "Paired: yes") {
		out, err := m.run(40*time.Second, "pair", mac)
		if err != nil && !strings.Contains(out, "AlreadyExists") {
			return failure("Pairing", out)
		}
	}
	m.run(5*time.Second, "trust", mac)
	out, err := m.run(30*time.Second, "connect", mac)
	if err != nil || !strings.Contains(out, "Connection successful") {
		return failure("Connecting", out)
	}
	return nil
}

func (m *Manager) Disconnect(mac string) error {
	if err := m.check(mac); err != nil {
		return err
	}
	if out, err := m.run(15*time.Second, "disconnect", mac); err != nil {
		return failure("Disconnecting", out)
	}
	return nil
}

// Remove forgets a device.
func (m *Manager) Remove(mac string) error {
	if err := m.check(mac); err != nil {
		return err
	}
	if out, err := m.run(15*time.Second, "remove", mac); err != nil {
		return failure("Removing", out)
	}
	return nil
}
