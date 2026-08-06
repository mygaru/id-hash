package hashenc

import (
	"flag"
	"github.com/mygaru/id-hash/cmd/id-hash/internal/hashenc/mock"
	"github.com/mygaru/id-hash/cmd/id-hash/internal/hashenc/sha256"
	"log"
	"sync"
)

var (
	function = flag.String("function", "sha256", "sha256")
	key      = flag.String("key", "", "Used as salt for sha256. Used as key for aes256.")
)

var (
	once sync.Once
	he   HashEnc
)

const keylen = 64

type HashEnc interface {
	// GenerateResult either via hashing or encryption, depending on the -function flag
	GenerateResult([]byte) ([]byte, error)
}

func Get() HashEnc {
	once.Do(func() {
		if *function == "" {
			log.Fatal("hashenc: missing required flag: -function")
		}

		if *key == "" {
			log.Fatal("hashenc: missing required flag: -key")
		}

		if len(*key) != keylen {
			log.Fatal("hashenc: invalid length: -key length must equal 64")
		}

		switch *function {

		case "sha256":
			he = sha256.New([]byte(*key))

		case "mock":
			he = mock.New([]byte(*key))

		default:
			log.Fatalf("hashenc: unsupported hash function: %s", *function)
		}

	})
	return he
}
