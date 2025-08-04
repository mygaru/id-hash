package core

import (
	"github.com/cespare/xxhash/v2"
	"strconv"
)

// ProcessPimBatch is responsible for iterating through the phones coming from a DCR Client (aka Data Vendor).
// Check if they exist in the Telecom's DB, and map it to a corresponding value unique for each phone (telco ident value)
func ProcessPimBatch(incomingBatch [][2]string) (batchToUidmap [][2]string, err error) {
	// Example implementation, where we map each phone to a hash of itself

	mapped := make([][2]string, 0, len(incomingBatch))

	for _, entry := range incomingBatch {
		phone := entry[0]
		token := entry[1]

		telcoIdent := strconv.FormatUint(xxhash.Sum64([]byte(phone)), 10)

		mapped = append(mapped, [2]string{telcoIdent, token})
	}

	return mapped, nil
}
