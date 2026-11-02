// mockhub serves fake Glances (port 61208), Beszel (8090), Vantage (8106)
// and Subsonic (4533) replies on 127.0.0.1, for trying MiniRack and
// MiniTape on a computer without real servers. MOCK_NOTOKEN=1 makes the
// Subsonic server refuse token logins (as with LDAP accounts), and
// MOCK_STREAM=FILE serves FILE for every stream. Not shipped.
package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

func js(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func main() {
	g := http.NewServeMux()
	start := time.Now()
	g.HandleFunc("/api/4/quicklook", func(w http.ResponseWriter, r *http.Request) {
		t := time.Since(start).Seconds()
		js(w, map[string]any{"cpu": 30 + 18*math.Sin(t/2.5) + 6*math.Sin(t*1.7), "mem": 61.2})
	})
	g.HandleFunc("/api/4/load", func(w http.ResponseWriter, r *http.Request) {
		js(w, map[string]any{"min1": 0.82, "min5": 0.64, "min15": 0.51})
	})
	g.HandleFunc("/api/4/uptime", func(w http.ResponseWriter, r *http.Request) { js(w, "12 days, 3:41:07") })
	g.HandleFunc("/api/4/fs", func(w http.ResponseWriter, r *http.Request) {
		js(w, []map[string]any{{"mnt_point": "/", "percent": 47.0, "size": 250e9, "used": 117e9}, {"mnt_point": "/srv/media", "percent": 91.3, "size": 4e12, "used": 3.65e12}})
	})
	g.HandleFunc("/api/4/sensors", func(w http.ResponseWriter, r *http.Request) {
		js(w, []map[string]any{{"label": "Package id 0", "value": 54.0, "type": "temperature_core"}, {"label": "nvme0", "value": 41.0, "type": "temperature_core"}})
	})
	go func() { log.Fatal(http.ListenAndServe("127.0.0.1:61208", g)) }()

	b := http.NewServeMux()
	b.HandleFunc("/api/collections/users/auth-with-password", func(w http.ResponseWriter, r *http.Request) { js(w, map[string]any{"token": "T"}) })
	b.HandleFunc("/api/collections/systems/records", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "T" {
			w.WriteHeader(401)
			return
		}
		js(w, map[string]any{"items": []map[string]any{
			{"name": "homeserver", "status": "up", "info": map[string]any{"cpu": 7.5, "mp": 38.0, "dp": 72.0, "la": []float64{0.3, 0.2, 0.1}, "u": 864000, "dt": 47.0}},
			{"name": "laptop", "status": "down", "info": map[string]any{}},
			{"name": "gaming-pc", "status": "up", "info": map[string]any{"cpu": 92.0, "mp": 81.0, "dp": 55.0, "la": []float64{3.1, 2.9, 2.2}, "u": 7200, "dt": 78.0}},
		}})
	})
	go func() { log.Fatal(http.ListenAndServe("127.0.0.1:8090", b)) }()

	v := http.NewServeMux()
	v.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		js(w, map[string]any{"host": map[string]any{"cpu": 12.0, "load": []float64{0.5, 0.4, 0.3}, "uptime": 3.2e6, "mem_total": 32e6, "mem_avail": 20e6, "temp": 49.0},
			"disks":    []map[string]any{{"name": "root", "total": 500e9, "used": 210e9}},
			"services": []map[string]any{{"name": "jellyfin", "status": "up"}, {"name": "navidrome", "status": "up"}, {"name": "backup", "status": "down"}},
			"peers":    map[string]any{}})
	})
	go func() { log.Fatal(http.ListenAndServe("127.0.0.1:8106", v)) }()

	s := http.NewServeMux()
	s.HandleFunc("/rest/", func(w http.ResponseWriter, r *http.Request) {
		ep := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/rest/"), ".view")
		if os.Getenv("MOCK_NOTOKEN") != "" && r.URL.Query().Get("t") != "" {
			js(w, map[string]any{"subsonic-response": map[string]any{"status": "failed", "version": "1.16.1",
				"error": map[string]any{"code": 41, "message": "Token authentication not supported for LDAP users."}}})
			return
		}
		ok := func(body map[string]any) {
			body["status"] = "ok"
			body["version"] = "1.16.1"
			js(w, map[string]any{"subsonic-response": body})
		}
		switch ep {
		case "ping":
			ok(map[string]any{})
		case "getAlbumList2":
			ok(map[string]any{"albumList2": map[string]any{"album": []map[string]any{
				{"id": "al1", "name": "Night Signals", "artist": "The Static Gardens", "year": 2007, "songCount": 10},
				{"id": "al2", "name": "Low Orbit", "artist": "Paper Satellites", "year": 2001, "songCount": 14},
				{"id": "al3", "name": "Quiet Machines", "artist": "Harbour Lights", "year": 1998, "songCount": 11}}}})
		case "getAlbum":
			ok(map[string]any{"album": map[string]any{"id": "al1", "name": "Night Signals", "artist": "The Static Gardens", "song": []map[string]any{
				{"id": "s1", "title": "Signal Fire", "artist": "The Static Gardens", "album": "Night Signals", "duration": 237, "track": 1, "coverArt": "al1"},
				{"id": "s2", "title": "Glass Rivers", "artist": "The Static Gardens", "album": "Night Signals", "duration": 242, "track": 2, "coverArt": "al1"},
				{"id": "s3", "title": "Undertow", "artist": "The Static Gardens", "album": "Night Signals", "duration": 255, "track": 3, "coverArt": "al1"}}}})
		case "getArtists":
			ok(map[string]any{"artists": map[string]any{"index": []map[string]any{{"name": "D", "artist": []map[string]any{{"id": "ar1", "name": "Paper Satellites", "albumCount": 4}}},
				{"name": "R", "artist": []map[string]any{{"id": "ar2", "name": "The Static Gardens", "albumCount": 9}}}}}})
		case "getRandomSongs":
			ok(map[string]any{"randomSongs": map[string]any{"song": []map[string]any{
				{"id": "s2", "title": "Glass Rivers", "artist": "The Static Gardens", "album": "Night Signals", "duration": 242, "coverArt": "al1"},
				{"id": "s9", "title": "Northbound", "artist": "Paper Satellites", "album": "Low Orbit", "duration": 320, "coverArt": "al2"}}}})
		case "getStarred2":
			ok(map[string]any{"starred2": map[string]any{"album": []map[string]any{{"id": "al3", "name": "Quiet Machines", "artist": "Harbour Lights", "year": 1998}},
				"song": []map[string]any{{"id": "s5", "title": "Static Bloom", "artist": "Harbour Lights", "duration": 330}}}})
		case "stream":
			if f := os.Getenv("MOCK_STREAM"); f != "" {
				http.ServeFile(w, r, f)
				return
			}
			w.WriteHeader(404)
		case "getCoverArt":
			img := image.NewRGBA(image.Rect(0, 0, 240, 240))
			for y := 0; y < 240; y++ {
				for x := 0; x < 240; x++ {
					img.Set(x, y, color.RGBA{uint8(x), uint8(60 + y/3), uint8(200 - x/2), 255})
				}
			}
			w.Header().Set("Content-Type", "image/png")
			png.Encode(w, img)
		case "getPlaylists":
			ok(map[string]any{"playlists": map[string]any{"playlist": []map[string]any{{"id": "p1", "name": "Road trip", "songCount": 42}}}})
		default:
			ok(map[string]any{})
		}
	})
	log.Fatal(http.ListenAndServe("127.0.0.1:4533", s))
}
