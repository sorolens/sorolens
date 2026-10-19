package soroban

import (
	"encoding/base64"
	"testing"
)

// FuzzDecodeScVal tests that DecodeScVal never panics on arbitrary string inputs
// and produces consistent representations for valid ScVal XDR encodings.
func FuzzDecodeScVal(f *testing.F) {
	// Seed corpus with known valid ScVal base64 encodings.
	seeds := []string{
		// Void
		"AAAAAQ==",
		// Bool
		"AAAAAAAAAAE=",
		"AAAAAAAAAAA=",
		// U32 / I32
		"AAAAAwAAAAo=",
		"AAAAAwAAAAA=",
		"AAAAA/////8=",
		"AAAAAwAAAAQ=",
		"AAAAAgAAAAAAAAAA",
		// U64 / I64 / Timepoint / Duration
		"AAAABAAAAAAAAAAq",
		"AAAABwAAAAAAAAAq",
		"AAAABQAAAAAAAAAq",
		"AAAABf//////////",
		"AAAABgAAAAAAAAAq",
		// 128-bit / 256-bit numerics
		"AAAACQAAAAAAAAAAAAAAAAAAAAE=",
		"AAAACv////////////////////8=",
		"AAAACwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB",
		"AAAADP//////////////////////////////////////////",
		// Symbols & Strings
		"AAAADwAAAAhUcmFuc2Zlcg==",
		"AAAADwAAAARtaW50",
		"AAAADwAAAAA=",
		"AAAADgAAAANhYmMA",
		// Bytes
		"AAAADQAAAALerQAA",
		// Collections (Vec / Map)
		"AAAAEAAAAAIAAAADAAAABQAAAAMAAAAG",
		"AAAAEQAAAAEAAAAPAAAAAWsAAAAAAAADAAAABw==",
		// Address
		"AAAAEgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==",
		"AAAAEgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==",
		// Boundary & invalid cases
		"",
		"!!!",
		"AAAA",
		"A",
		"AA==",
		"AAA=",
		"----",
		base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 0, 0, 0, 0, 1}),
		base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 3, 0, 0, 0, 42}),
	}

	for _, s := range seeds {
		f.Add(s)
	}

	// FuzzDecodeScVal executes without recovering panics so any decoder parser bugs or
	// unexpected crashes directly trigger a test failure as required by issue #552.
	f.Fuzz(func(t *testing.T, b64 string) {
		got, err := DecodeScVal(b64)
		if err != nil {
			// Expected for malformed base64 or buffers shorter than 4 bytes.
			return
		}

		// Invariants for successfully decoded ScVal
		if got.Type == "" {
			t.Errorf("DecodeScVal returned empty Type for input: %q", b64)
		}
		if got.Human == "" && got.Type != "scvString" && got.Type != "scvSymbol" && got.Type != "scvBytes" {
			t.Errorf("DecodeScVal returned empty Human representation for type %s", got.Type)
		}
	})
}

// TestDecodeScVal_RandomizedFuzzRegression runs a short randomized fuzzing pass
// during standard "go test" CI execution to satisfy continuous fuzzing regression criteria.
func TestDecodeScVal_RandomizedFuzzRegression(t *testing.T) {
	seeds := []string{
		"AAAAAQ==",
		"AAAAAAAAAAE=",
		"AAAAAwAAAAo=",
		"AAAABAAAAAAAAAAq",
		"AAAADwAAAAhUcmFuc2Zlcg==",
		"AAAADQAAAALerQAA",
		"AAAAEAAAAAIAAAADAAAABQAAAAMAAAAG",
	}

	for i := 0; i < 500; i++ {
		var input string
		switch i % 4 {
		case 0:
			// Corrupted seed
			s := seeds[i%len(seeds)]
			idx := (i * 7) % len(s)
			input = s[:idx] + "!" + s[idx+1:]
		case 1:
			// Truncated seed
			s := seeds[i%len(seeds)]
			cut := (i * 3) % len(s)
			input = s[:cut]
		case 2:
			// Deterministic pseudo-random bytes
			b := make([]byte, (i%32)+1)
			for j := range b {
				b[j] = byte((i*31 + j*17) & 0xff)
			}
			input = base64.StdEncoding.EncodeToString(b)
		default:
			// Arbitrary characters
			input = string(rune(i)) + "==="
		}

		got, err := DecodeScVal(input)
		if err != nil {
			continue
		}
		if got.Type == "" {
			t.Errorf("DecodeScVal returned empty Type for input: %q", input)
		}
	}
}
