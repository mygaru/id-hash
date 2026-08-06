package mock

type MockHF struct {
	salt []byte
}

func New(salt []byte) *MockHF {
	return &MockHF{salt: salt}
}

func (m *MockHF) GenerateResult(b []byte) ([]byte, error) {
	// Simulating that GenerateResult always returns a specific base64 string for a given MSISDN
	// e.g., "12345" hashes and encodes to "bW9ja0I2NFJlc3VsdA=="
	if string(b) == "1234567890" {
		return []byte("bW9ja0I2NFJlc3VsdA=="), nil
	}
	return []byte("YW5vdGhlckI2NA=="), nil
}
