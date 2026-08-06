package core

import (
	"flag"
	"log"
	"reflect"
	"testing"
)

func TestProcessPimBatch(t *testing.T) {
	flag.Set("function", "mock")
	flag.Set("key", "FoHlAH3.YCF70C7?W&HV0pC03wn+&cUmRfNOe1sHUQ;(VKIPf:<UW$Sk!8nriQ4/")
	flag.Parse()

	// NOTE: You will need to swap out how `hashenc.Get()` is mocked
	// in your actual code if it uses a global state.

	tests := []struct {
		name          string
		incomingBatch [][2]string
		wantMap       [][2]string
		wantErr       bool
	}{
		{
			name: "Condition 1: Valid numeric MSISDN (<= 15 digits) should be hashed (b64 output)",
			incomingBatch: [][2]string{
				{"1234567890", "token1"},
			},
			wantMap: [][2]string{
				{"bW9ja0I2NFJlc3VsdA==", "token1"}, // output of mock hash
			},
			wantErr: false,
		},
		{
			name: "Condition 1: Numeric but > 15 digits should be left alone",
			incomingBatch: [][2]string{
				{"1234567890123456", "token2"}, // 16 digits (Valid Hex too, so it gets hex-decoded)
			},
			wantMap: [][2]string{
				{"1234567890123456", "token2"}, // hex.Decode("1234567890123456") -> b64 encoded
			},
			wantErr: false,
		},
		{
			name: "Condition 2: random non-msisdn values should be left alone",
			incomingBatch: [][2]string{
				{"4a3f2b", "token3"}, // "4a3f2b" in hex is [74, 63, 43]
			},
			wantMap: [][2]string{
				{"4a3f2b", "token3"},
			},
			wantErr: false,
		},
		{
			name: "Condition 3: Already Base64 string should be left alone",
			incomingBatch: [][2]string{
				{"SGVsbG8gV29ybGQ=", "token4"}, // Valid Base64 text ("Hello World") but NOT valid Hex
			},
			wantMap: [][2]string{
				{"SGVsbG8gV29ybGQ=", "token4"}, // Unchanged
			},
			wantErr: false,
		},
		{
			name: "Condition 4: Raw arbitrary string (fallback) should be left alone",
			incomingBatch: [][2]string{
				{"plain-text-string!", "token5"}, // Not numeric, not hex, not base64 (contains '-')
			},
			wantMap: [][2]string{
				{"plain-text-string!", "token5"}, // Plain string standard Base64 encoded
			},
			wantErr: false,
		},
		{
			name: "Mixed Batch: Handles multiple types in one run",
			incomingBatch: [][2]string{
				{"1234567890", "t1"},
				{"4a3f2b", "t2"},
				{"SGVsbG8gV29ybGQ=", "t3"},
			},
			wantMap: [][2]string{
				{"bW9ja0I2NFJlc3VsdA==", "t1"},
				{"4a3f2b", "t2"},
				{"SGVsbG8gV29ybGQ=", "t3"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// If you use a global mock injector for hashenc, set it up here.

			gotMap, err := ProcessPimBatch(tt.incomingBatch)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessPimBatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			log.Printf("%v", gotMap)
			if !reflect.DeepEqual(gotMap, tt.wantMap) {
				t.Errorf("ProcessPimBatch() = %v, want %v", gotMap, tt.wantMap)
			}
		})
	}
}
