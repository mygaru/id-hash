package sha256

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

type SHA256 struct {
	key []byte
}

func New(key []byte) *SHA256 {
	return &SHA256{key: key}
}

func (s *SHA256) GenerateResult(plaintxt []byte) ([]byte, error) {
	h := hmac.New(sha256.New, s.key)
	h.Write(plaintxt)
	hm := h.Sum(nil)
	res := make([]byte, base64.StdEncoding.EncodedLen(len(hm)))

	base64.StdEncoding.Encode(res, hm)
	return res, nil
}
