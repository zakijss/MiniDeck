package tnet

import "strings"

// NormalizeURL turns a server address as typed into a URL. A bare host
// ("homeserver", "nas:8080") gets http:// and, when it has no port, the
// app's default port. An address typed with its scheme is taken as given:
// https://music.example.com means port 443, not the default.
func NormalizeURL(u, port string) string {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	if u == "" || strings.Contains(u, "://") {
		return u
	}
	if !strings.Contains(u, ":") && !strings.Contains(u, "/") && port != "" {
		u += ":" + port
	}
	return "http://" + u
}

// HostChoices lists the tailnet's devices for a server picker
// (ui.Screen.PickHost): their names, and beside each its OS and whether
// it is offline.
func HostChoices() (names, details []string, err error) {
	peers, err := Peers()
	if err != nil {
		return nil, nil, err
	}
	for _, p := range peers {
		d := p.OS
		if !p.Online {
			d += ", offline"
		}
		names = append(names, p.Name)
		details = append(details, d)
	}
	return names, details, nil
}
