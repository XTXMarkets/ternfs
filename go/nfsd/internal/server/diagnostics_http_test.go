// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
)

func getJSON(t *testing.T, base, path string, want int) map[string]any {
	t.Helper()
	client := http.Client{Timeout: testutil.ChannelTimeout}
	resp, err := client.Get(base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("GET %s = %d, want %d: %s", path, resp.StatusCode, want, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET %s content type %q", path, ct)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("GET %s: %v: %s", path, err, body)
	}
	for _, key := range []string{"now", "server_id", "stateid_epoch", "wire_epoch", "mode"} {
		if _, ok := out[key]; !ok {
			t.Fatalf("GET %s: no %s in envelope: %s", path, key, body)
		}
	}
	return out
}

// The HTTP surface: the index, each resource and its filters, ids that
// paste back between resources, JSON errors and pprof.
func TestDiagnosticsHTTP(t *testing.T) {
	f := newDiagnosticsFixture(t)
	ts := httptest.NewServer(inspect.Handler(f.srv, nil))
	defer ts.Close()

	index := getJSON(t, ts.URL, "/", http.StatusOK)
	resources := index["resources"].([]any)
	if len(resources) < 6 {
		t.Fatalf("index lists %d resources", len(resources))
	}
	needsServer := map[string]bool{}
	for _, r := range resources {
		r := r.(map[string]any)
		if r["path"] == "" || r["description"] == "" {
			t.Fatalf("resource %v lacks a path or description", r)
		}
		needsServer[r["path"].(string)] = r["needs_running_server"] == true
	}
	// The index must say what the handler does: lookup resolves against
	// memory and is unavailable in store-only mode; the store report is not.
	if !needsServer["/v1/lookup"] || needsServer["/v1/store"] || needsServer["/"] {
		t.Fatalf("needs_running_server = %v", needsServer)
	}
	if enums := index["enums"].(map[string]any); enums["client.state"] == nil || enums["id kinds"] == nil {
		t.Fatalf("enums = %v", enums)
	}

	status := getJSON(t, ts.URL, "/v1/status", http.StatusOK)
	if status["mode"] != inspect.ModeServing || status["accounting"].(map[string]any)["sessions"].(float64) != 1 {
		t.Fatalf("status = %v", status)
	}

	wire := wireText(f.wire)
	clients := getJSON(t, ts.URL, "/v1/clients?wire="+wire, http.StatusOK)
	recs := clients["records"].([]any)
	if len(recs) != 1 || recs[0].(map[string]any)["state"] != inspect.ClientConfirmed || recs[0].(map[string]any)["wire_clientid"] != wire {
		t.Fatalf("clients = %v", clients)
	}
	// Emitted ids paste back: the record's incarnation into the live
	// filters, and an open's stateid into lookup.
	incarnation := recs[0].(map[string]any)["incarnation"].(string)
	if incarnation != incarnationText(f.incarnID) {
		t.Fatalf("incarnation = %q", incarnation)
	}
	incs := getJSON(t, ts.URL, "/v1/incarnations?id="+incarnation, http.StatusOK)
	if recs := incs["records"].([]any); len(recs) != 1 || recs[0].(map[string]any)["group"] == nil {
		t.Fatalf("incarnations = %v", incs)
	}

	session := sessionText(f.session)
	sessions := getJSON(t, ts.URL, "/v1/sessions?id="+session, http.StatusOK)
	rec := sessions["records"].([]any)[0].(map[string]any)
	if rec["slots"] == nil {
		t.Fatalf("slots omitted by default: %v", rec)
	}
	sessions = getJSON(t, ts.URL, "/v1/sessions?slots=0", http.StatusOK)
	if rec := sessions["records"].([]any)[0].(map[string]any); rec["slots"] != nil {
		t.Fatalf("slots=0 still reports slots: %v", rec)
	}

	opens := getJSON(t, ts.URL, "/v1/opens?incarnation="+incarnation, http.StatusOK)
	if recs := opens["records"].([]any); len(recs) != 1 || opens["views"] == nil {
		t.Fatalf("opens = %v", opens)
	}
	stateid := opens["records"].([]any)[0].(map[string]any)["stateid"].(string)
	staging := getJSON(t, ts.URL, "/v1/staging", http.StatusOK)
	if recs := staging["records"].([]any); len(recs) != 1 || staging["configured"] != true {
		t.Fatalf("staging = %v", staging)
	}
	confs := getJSON(t, ts.URL, "/v1/confirmations", http.StatusOK)
	if len(confs["records"].([]any)) != 0 {
		t.Fatalf("confirmations = %v", confs)
	}

	conns := getJSON(t, ts.URL, "/v1/connections?session="+session, http.StatusOK)
	if recs := conns["records"].([]any); len(recs) != 1 || recs[0].(map[string]any)["path"] == nil {
		t.Fatalf("connections = %v", conns)
	}

	lookup := getJSON(t, ts.URL, "/v1/lookup?id="+stateid, http.StatusOK)
	if lookup["kind"] != inspect.KindStateid || lookup["known"] != true || lookup["id"] != stateid {
		t.Fatalf("lookup = %v", lookup)
	}

	// A serving nfsd keeps no client store to report on.
	getJSON(t, ts.URL, "/v1/store?incarnation="+incarnation, http.StatusServiceUnavailable)

	// Errors are JSON with the envelope and a status.
	for path, want := range map[string]int{
		"/v1/lookup":                       http.StatusBadRequest,
		"/v1/lookup?id=zz":                 http.StatusBadRequest,
		"/v1/lookup?id=0x4000000100000001": http.StatusBadRequest, // untyped
		"/v1/lookup?id=" + inspect.ID(inspect.KindIncarnation, make([]byte, 8)): http.StatusBadRequest, // zero
		"/v1/clients?wire=" + incarnation:                                       http.StatusBadRequest, // wrong kind, same width
		"/v1/sessions?id=" + wire:                                               http.StatusBadRequest,
		"/v1/opens?stateid=":                                                    http.StatusBadRequest,
		"/nope":                                                                 http.StatusNotFound,
	} {
		out := getJSON(t, ts.URL, path, want)
		if out["error"] == "" || out["status"].(float64) != float64(want) {
			t.Fatalf("GET %s = %v", path, out)
		}
	}
	resp, err := http.Post(ts.URL+"/v1/status", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d", resp.StatusCode)
	}
	resp, err = http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pprof index = %d", resp.StatusCode)
	}
}

// Without a running server or a store the API still answers the index with
// its fallback envelope and marks the memory resources unavailable,
// lookup included, whether or not it was given an id.
func TestDiagnosticsHTTPStoreOnly(t *testing.T) {
	ts := httptest.NewServer(inspect.Handler(nil, nil))
	defer ts.Close()
	index := getJSON(t, ts.URL, "/", http.StatusOK)
	if index["mode"] != inspect.ModeStoreOnly || index["server_id"] != "" {
		t.Fatalf("envelope = %v", index)
	}
	for _, r := range index["resources"].([]any) {
		r := r.(map[string]any)
		if r["needs_running_server"] == true {
			getJSON(t, ts.URL, r["path"].(string), http.StatusServiceUnavailable)
		}
	}
	for _, path := range []string{"/v1/lookup", "/v1/lookup?id=" + incarnationText(1), "/v1/store"} {
		getJSON(t, ts.URL, path, http.StatusServiceUnavailable)
	}
}
