package sha256

import (
	"crypto/sha256"
	"encoding/base64"
)

type SHA256 struct {
	salt []byte
}

func New(salt []byte) *SHA256 {
	return &SHA256{salt: salt}
}

func (s *SHA256) GenerateResult(plaintxt []byte) ([]byte, error) {
	sha := sha256.New()
	sha.Write(s.salt)
	sha.Write(plaintxt)
	h := sha.Sum(nil)
	res := make([]byte, base64.StdEncoding.EncodedLen(len(h)))

	base64.StdEncoding.Encode(res, sha.Sum(nil))
	return res, nil
}
