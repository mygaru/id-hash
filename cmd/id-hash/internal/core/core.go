package core

import (
	"github.com/mygaru/id-hash/cmd/id-hash/internal/hashenc"
)

// ProcessPimBatch is responsible for iterating through the phones coming from a DCR Client (aka Data Vendor).
// Check if they exist in the Telecom's DB, and map it to a corresponding value unique for each phone (telco hash)
func ProcessPimBatch(incomingBatch [][2]string) (batchToUidmap [][2]string, err error) {
	// Example implementation, where we map each phone to a hash of itself
	hashfunc := hashenc.Get()

	mapped := make([][2]string, 0, len(incomingBatch))

	for _, entry := range incomingBatch {
		phone := entry[0]
		token := entry[1]

		telcoIdent, err := hashfunc.GenerateResult([]byte(phone))
		if err != nil {
			return nil, err
		}

		mapped = append(mapped, [2]string{string(telcoIdent), token})
	}

	return mapped, nil
}
