package crc

import (
	"encoding/hex"
	"testing"
)

// Golden frames pulled directly from Pictor's protocol PDFs (Appendix B).
// These prove the lookup table + algorithm exactly match the vendor's reference.
func TestSum_GoldenFrames(t *testing.T) {
	cases := []struct {
		name      string
		hexBefore string // bytes from "Packet Length" through "Serial Number" inclusive
		want      uint16
	}{
		{
			// PS10A login: 78 78 0D 01 03 53 41 35 32 15 03 62 00 02 [2D 06] 0D 0A
			name:      "PS10A login",
			hexBefore: "0D 01 03 53 41 35 32 15 03 62 00 02",
			want:      0x2D06,
		},
		{
			// PS10A login response: 78 78 05 01 00 02 [EB 47] 0D 0A
			name:      "PS10A login response",
			hexBefore: "05 01 00 02",
			want:      0xEB47,
		},
		{
			// PS10A status: 78 78 0A 13 44 01 04 00 01 00 05 [08 45] 0D 0A
			name:      "PS10A status",
			hexBefore: "0A 13 44 01 04 00 01 00 05",
			want:      0x0845,
		},
		{
			// PS10A status response: 78 78 05 13 00 05 [AF D5] 0D 0A
			name:      "PS10A status response",
			hexBefore: "05 13 00 05",
			want:      0xAFD5,
		},
		{
			// PS10A login example from §5.1.3:
			// 78 78 0D 01 01 23 45 67 89 01 23 45 00 01 [8C DD] 0D 0A
			name:      "PS10A login (spec §5.1.3)",
			hexBefore: "0D 01 01 23 45 67 89 01 23 45 00 01",
			want:      0x8CDD,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := hex.DecodeString(stripSpaces(tc.hexBefore))
			if err != nil {
				t.Fatalf("decode hex: %v", err)
			}
			got := Sum(b)
			if got != tc.want {
				t.Fatalf("got %04X, want %04X", got, tc.want)
			}
		})
	}
}

func stripSpaces(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			out = append(out, s[i])
		}
	}
	return string(out)
}
