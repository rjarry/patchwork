// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import "testing"

func TestTruncStr(t *testing.T) {
	tests := []struct {
		input   string
		max     int
		want    string
		wantLen int
	}{
		{"hello", 10, "hello", 5},
		{"hello", 5, "hello", 5},
		{"hello world", 8, "hello…", 8},
		// don't split a 2-byte rune (é = 0xc3 0xa9)
		{"café latte", 6, "caf…", 6},
		// don't split a 3-byte rune (日 = 0xe6 0x97 0xa5)
		{"日本語テスト", 10, "日本…", 9},
		// don't split a 4-byte rune (🎉 = 0xf0 0x9f 0x8e 0x89)
		{"🎉🎉🎉", 8, "🎉…", 7},
		// very short limit: no room for ellipsis, hard cut
		{"hello", 3, "hel", 3},
		{"hello", 4, "h…", 4},
	}

	for _, tt := range tests {
		got := truncStr(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("truncStr(%q, %d) = %q, want %q",
				tt.input, tt.max, got, tt.want)
		}
		if len(got) > tt.max {
			t.Errorf("truncStr(%q, %d): len=%d exceeds max",
				tt.input, tt.max, len(got))
		}
	}
}
