// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
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
	for _, key := range []string{"now", "nfsd_id", "stateid_epoch", "wire_epoch", "mode"} {
		if _, ok := out[key]; !ok {
			t.Fatalf("GET %s: no %s in envelope: %s", path, key, body)
		}
	}
	return out
}

// The HTTP surface: the index, each resource and its filters, ids that
// paste back between resources, the store report, JSON errors and pprof.
func TestDiagnosticsHTTP(t *testing.T) {
	f := newDiagnosticsFixture(t)
	ts := httptest.NewServer(inspect.Handler(f.srv, f.srv))
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
	// Emitted ids paste back: the record's incarnation into the live filters
	// and the store query below, and an open's stateid into lookup.
	incarnation := recs[0].(map[string]any)["incarnation"].(string)
	if incarnation != incarnationText(f.incarnID) {
		t.Fatalf("incarnation = %q", incarnation)
	}
	incs := getJSON(t, ts.URL, "/v1/incarnations?id="+incarnation, http.StatusOK)
	if recs := incs["records"].([]any); len(recs) != 1 || recs[0].(map[string]any)["lease_refresher"] == nil {
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

	store := getJSON(t, ts.URL, "/v1/store?incarnation="+incarnation, http.StatusOK)
	report := store["report"].(map[string]any)
	if ids := report["identities"].([]any); len(ids) != 1 {
		t.Fatalf("store report = %v", report)
	}
	if report["staging_dir"] != nil {
		t.Fatal("staging read without staging=1")
	}
	store = getJSON(t, ts.URL, "/v1/store?incarnation="+incarnation+"&staging=1", http.StatusOK)
	if store["report"].(map[string]any)["staging_dir"] == nil {
		t.Fatal("staging=1 did not read the staging directory")
	}

	// Errors are JSON with the envelope and a status.
	for path, want := range map[string]int{
		"/v1/lookup":                       http.StatusBadRequest,
		"/v1/lookup?id=zz":                 http.StatusBadRequest,
		"/v1/lookup?id=0x4000000100000001": http.StatusBadRequest, // untyped
		"/v1/lookup?id=" + inspect.ID(inspect.KindIncarnation, make([]byte, 8)): http.StatusBadRequest, // zero
		"/v1/clients?wire=" + incarnation:                                       http.StatusBadRequest, // wrong kind, same width
		"/v1/sessions?id=" + wire:                                               http.StatusBadRequest,
		"/v1/store?incarnation=" + wire:                                         http.StatusBadRequest,
		"/v1/store?incarnation=":                                                http.StatusBadRequest, // present but empty
		"/v1/opens?stateid=":                                                    http.StatusBadRequest,
		"/v1/store?incarnation=" + incarnation + "&stateid=" + stateid:          http.StatusBadRequest, // two selectors
		"/nope": http.StatusNotFound,
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
	if index["mode"] != inspect.ModeStoreOnly || index["nfsd_id"] != "" {
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

// Store-only mode reads the client store and the staging directory of a
// stopped server and changes nothing on disk.
func TestDiagnosticsStoreOnlyMode(t *testing.T) {
	root, stagingDir := t.TempDir(), t.TempDir()
	fs := vfs.NewLocalTernVFS(root)
	ss, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, ss, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	conn := dial(t, addr)
	xid := uint32(1)
	wire := setupClient(t, conn, &xid)
	openCreateFile(t, conn, &xid, wire, "f") // leaves a staged writer on disk
	incarnation := binary.BigEndian.Uint64(testSessionForClient(conn, wire).id[4:12])
	conn.Close()
	cleanup()

	// A metadata temporary, as a crash during a sidecar save leaves behind.
	if err := os.WriteFile(filepath.Join(stagingDir, ".0000000000000001.meta.tmp-x"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	listing := func() string {
		entries, err := os.ReadDir(stagingDir)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&b, "%s %d %d\n", e.Name(), info.Size(), info.ModTime().UnixNano())
		}
		return b.String()
	}
	before := listing()
	if !strings.Contains(before, ".staging") || !strings.Contains(before, ".meta ") {
		t.Fatalf("no staged writer on disk:\n%s", before)
	}

	store, err := clientstore.OpenReader(vfs.NewLocalTernVFS(root))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(inspect.Handler(nil, StoreOnlySource{ClientStore: store, StagingDir: stagingDir}))
	defer ts.Close()

	index := getJSON(t, ts.URL, "/", http.StatusOK)
	if index["mode"] != inspect.ModeStoreOnly || index["nfsd_id"] != "" {
		t.Fatalf("envelope = %v", index)
	}
	getJSON(t, ts.URL, "/v1/sessions", http.StatusServiceUnavailable)
	getJSON(t, ts.URL, "/v1/lookup?id="+wireText(wire), http.StatusServiceUnavailable)

	report := getJSON(t, ts.URL, "/v1/store?incarnation="+incarnationText(incarnation)+"&staging=1", http.StatusOK)["report"].(map[string]any)
	if ids := report["identities"].([]any); len(ids) != 1 {
		t.Fatalf("identities = %v", ids)
	}
	// A sidecar is joined to its open marker when one exists, otherwise
	// listed under the incarnation or, unowned, at the top level.
	staged := 0
	count := func(v any) {
		if list, ok := v.([]any); ok {
			staged += len(list)
		}
	}
	for _, inc := range report["identities"].([]any)[0].(map[string]any)["incarnations"].([]any) {
		inc := inc.(map[string]any)
		count(inc["staging"])
		if opens, ok := inc["opens"].([]any); ok {
			for _, open := range opens {
				if open.(map[string]any)["staging"] != nil {
					staged++
				}
			}
		}
	}
	count(report["staging"])
	if staged != 1 {
		t.Fatalf("report shows %d staged writers, want 1: %v", staged, report)
	}
	problems, _ := report["problems"].([]any)
	found := false
	for _, p := range problems {
		found = found || strings.Contains(p.(string), "temporary metadata file present")
	}
	if !found {
		t.Fatalf("temporary not reported: %v", problems)
	}
	if after := listing(); after != before {
		t.Fatalf("store-only mode changed the staging directory:\n%s\n--\n%s", before, after)
	}
}
