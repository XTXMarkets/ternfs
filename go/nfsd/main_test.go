// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"strings"
	"testing"
)

func TestParseInspectOptions(t *testing.T) {
	hash := strings.Repeat("a", 64)
	tests := []struct {
		name                                      string
		identity, identityHash, clientID, stateID string
		wantErr                                   bool
	}{
		{name: "no filter"},
		{name: "identity", identity: "client"},
		{name: "identity hash", identityHash: hash},
		{name: "uppercase identity hash", identityHash: strings.ToUpper(hash)},
		{name: "clientid", clientID: "0x2000000000000001"},
		{name: "stateid", stateID: strings.Repeat("0", 24)},
		{name: "zero clientid", clientID: "0", wantErr: true},
		{name: "file inode as clientid", clientID: "0x4000000000000001", wantErr: true},
		{name: "short identity hash", identityHash: "abcd", wantErr: true},
		{name: "invalid identity hash", identityHash: strings.Repeat("g", 64), wantErr: true},
		{name: "two filters", identity: "client", clientID: "0x2000000000000001", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts, err := parseInspectOptions(
				test.identity, test.identityHash, test.clientID, test.stateID)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseInspectOptions() error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && test.identityHash != "" &&
				opts.IdentityHash != strings.ToLower(test.identityHash) {
				t.Fatalf("identity hash = %q", opts.IdentityHash)
			}
		})
	}
}
