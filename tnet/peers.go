package tnet

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Peer is a device on the tailnet, as the setup screens list it.
type Peer struct {
	Name   string // MagicDNS short name, usable in URLs
	OS     string
	Online bool
}

// Peers lists the other devices on the tailnet, online ones first, using
// the minideck app's tailscale CLI. MINIDECK_PEERS_JSON (a file holding
// `tailscale status --json` output) replaces it for tests.
func Peers() ([]Peer, error) {
	var out []byte
	var err error
	if f := os.Getenv("MINIDECK_PEERS_JSON"); f != "" {
		out, err = os.ReadFile(f)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err = exec.CommandContext(ctx, "/tmp/miniscale/tailscale",
			"--socket=/tmp/miniscale/tailscaled.sock", "status", "--json").Output()
		if err != nil {
			return nil, sentence("MiniScale is off. Turn it on in Apps > MiniScale, or type the address.")
		}
	}
	if err != nil {
		return nil, err
	}
	var st struct {
		BackendState string
		Peer         map[string]struct {
			HostName string
			DNSName  string
			OS       string
			Online   bool
		}
	}
	if err := json.Unmarshal(out, &st); err != nil {
		return nil, err
	}
	var peers []Peer
	for _, p := range st.Peer {
		name := strings.SplitN(strings.TrimSuffix(p.DNSName, "."), ".", 2)[0]
		if name == "" {
			name = strings.ToLower(p.HostName)
		}
		peers = append(peers, Peer{Name: name, OS: p.OS, Online: p.Online})
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Online != peers[j].Online {
			return peers[i].Online
		}
		return peers[i].Name < peers[j].Name
	})
	if len(peers) == 0 {
		return nil, sentence("No other devices on your tailnet yet.")
	}
	return peers, nil
}
