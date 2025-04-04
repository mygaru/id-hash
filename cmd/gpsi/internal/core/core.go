package core

import (
	"github.com/cespare/xxhash/v2"
	"github.com/mygaru/uidmap/pkg/pim"
	"strconv"
)

// ProcessPimBatch is responsible for iterating through the phones (MSISDNs) coming from a DCR Client.
// Check if they exist in the Telecom's DB, and map it to a corresponding value unique for each phone
func ProcessPimBatch(request pim.MsisdnRequest, resp pim.GpsiRequest) (pim.GpsiRequest, error) {
	// Example implementation, where we map each phone to a hash of itself

	err := request.Iterate(func(phone, token []byte) error {
		h := strconv.FormatUint(xxhash.Sum64(phone), 10)
		resp.AddRow([]byte(h), token)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}
