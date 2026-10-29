// Copyright (c) 2026 contentways
// SPDX-License-Identifier: MIT

package solver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeAPI is a minimal Poweradmin API v2: zone lookup by name and the TXT
// RRset endpoints, with the zone scoping of an API key.
type fakeAPI struct {
	t      *testing.T
	apiKey string

	mu     sync.Mutex
	zones  map[int]string               // id -> name
	rrsets map[int]map[string]fakeRRSet // zone id -> "name|type" -> rrset
	puts   int
	// quoteContents makes the API return TXT contents with quotes, as
	// PowerDNS stores them.
	quoteContents bool
	// onGet runs before every RRset read, to simulate concurrent writers.
	onGet func()
	// status, when set, answers every authenticated request, to simulate
	// outages and missing permissions.
	status int
}

type fakeRRSet struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	TTL     int               `json:"ttl"`
	Records []fakeRecordEntry `json:"records"`
}

type fakeRecordEntry struct {
	Content  string `json:"content"`
	Priority int    `json:"priority"`
	Disabled bool   `json:"disabled"`
}

func newFakeAPI(t *testing.T, zones ...string) (*fakeAPI, *httptest.Server) {
	f := &fakeAPI{t: t, apiKey: "test-key", zones: map[int]string{}, rrsets: map[int]map[string]fakeRRSet{}}
	for i, z := range zones {
		f.zones[i+1] = z
		f.rrsets[i+1] = map[string]fakeRRSet{}
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+f.apiKey {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/")
	parts := strings.Split(path, "/")

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.status != 0 {
		writeError(w, f.status, http.StatusText(f.status))
		return
	}

	switch {
	case r.Method == http.MethodGet && path == "zones":
		name := r.URL.Query().Get("name")
		var zones []map[string]any
		for id, z := range f.zones {
			if name == "" || z == name {
				zones = append(zones, map[string]any{"id": id, "name": z, "type": "MASTER"})
			}
		}
		writeData(w, http.StatusOK, map[string]any{"zones": zones})

	case len(parts) >= 3 && parts[0] == "zones" && parts[2] == "rrsets":
		id, _ := strconv.Atoi(parts[1])
		zoneName, ok := f.zones[id]
		if !ok {
			writeError(w, http.StatusNotFound, "Zone not found")
			return
		}
		f.serveRRSets(w, r, id, zoneName, parts[3:])

	default:
		writeError(w, http.StatusNotFound, "Not found: "+r.Method+" "+r.URL.Path)
	}
}

func (f *fakeAPI) serveRRSets(w http.ResponseWriter, r *http.Request, zoneID int, zoneName string, rest []string) {
	switch {
	case r.Method == http.MethodGet && len(rest) == 2:
		if f.onGet != nil {
			hook := f.onGet
			f.mu.Unlock()
			hook()
			f.mu.Lock()
		}
		rr, ok := f.rrsets[zoneID][f.fqdn(rest[0], zoneName)+"|"+rest[1]]
		if !ok {
			writeError(w, http.StatusNotFound, "RRSet not found")
			return
		}
		if f.quoteContents {
			rr.Records = slices.Clone(rr.Records)
			for i := range rr.Records {
				rr.Records[i].Content = `"` + rr.Records[i].Content + `"`
			}
		}
		writeData(w, http.StatusOK, map[string]any{"rrset": rr})

	case r.Method == http.MethodPut && len(rest) == 0:
		var req fakeRRSet
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		for _, rec := range req.Records {
			if strings.HasPrefix(rec.Content, `"`) {
				f.t.Errorf("TXT content sent quoted: %q", rec.Content)
			}
		}
		f.puts++
		req.Name = f.fqdn(req.Name, zoneName)
		f.rrsets[zoneID][req.Name+"|"+req.Type] = req
		writeData(w, http.StatusOK, map[string]any{"rrset": req})

	case r.Method == http.MethodDelete && len(rest) == 2:
		key := f.fqdn(rest[0], zoneName) + "|" + rest[1]
		if _, ok := f.rrsets[zoneID][key]; !ok {
			writeError(w, http.StatusNotFound, "RRSet not found")
			return
		}
		delete(f.rrsets[zoneID], key)
		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// fqdn mirrors Poweradmin, which accepts relative and absolute names.
func (f *fakeAPI) fqdn(name, zone string) string {
	name = strings.TrimSuffix(name, ".")
	if name == "@" || name == zone {
		return zone
	}
	if strings.HasSuffix(name, "."+zone) {
		return name
	}
	return name + "." + zone
}

func (f *fakeAPI) values(zoneID int, name string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	rr, ok := f.rrsets[zoneID][name+"|TXT"]
	if !ok {
		return nil
	}
	var out []string
	for _, r := range rr.Records {
		out = append(out, r.Content)
	}
	return out
}

func (f *fakeAPI) rrset(zoneID int, name string) (fakeRRSet, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rr, ok := f.rrsets[zoneID][name+"|TXT"]
	return rr, ok
}

func (f *fakeAPI) seed(zoneID int, rr fakeRRSet) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rrsets[zoneID][rr.Name+"|"+rr.Type] = rr
}

func writeData(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": msg, "data": nil})
}
