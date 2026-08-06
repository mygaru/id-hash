package core

import (
	"github.com/mygaru/id-hash/cmd/id-hash/internal/hashenc"
	"strings"
)

// ProcessPimBatch is responsible for iterating through the phones coming from a DCR Client (aka Data Vendor).
// Check if they exist in the Telecom's DB, and map it to a corresponding value unique for each phone (telco hash)
func ProcessPimBatch(incomingBatch [][2]string) (batchToUidmap [][2]string, err error) {
	isNotDigit := func(c rune) bool { return c < '0' || c > '9' }
	hashfunc := hashenc.Get()

	mapped := make([][2]string, 0, len(incomingBatch))

	for _, entry := range incomingBatch {
		value := entry[0]
		token := entry[1]

		// 1. MSISDN Check -> Hash it
		if len(value) <= 15 && !strings.ContainsFunc(value, isNotDigit) {
			telcoIdent, err := hashfunc.GenerateResult([]byte(value))
			if err != nil {
				return nil, err
			}
			mapped = append(mapped, [2]string{string(telcoIdent), token})
			continue
		}

		// 2. value is already hashed
		mapped = append(mapped, [2]string{value, token})
	}

	return mapped, nil
}
