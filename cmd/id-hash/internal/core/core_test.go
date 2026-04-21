package core

import (
	"flag"
	"github.com/google/uuid"
	"testing"
)

func TestPimBach(t *testing.T) {
	flag.Set("function", "sha256")
	flag.Set("key", "FoHlAH3.YCF70C7?W&HV0pC03wn+&cUmRfNOe1sHUQ;(VKIPf:<UW$Sk!8nriQ4/")
	flag.Parse()

	token := uuid.NewString()
	r, err := ProcessPimBatch([][2]string{{"h1", token}})
	if err != nil {
		t.Fatal(err)
	}

	if len(r) != 1 {
		t.Fatalf("len(r) = %d; want 1", len(r))
	}

	hash := r[0][0]
	if hash != "7ywvhLgYXQ+DYcLlAcQAXpoe58Bvqh8SiUu9reiLL/8=" {
		t.Fatalf("wrong hash: %s", hash)
	}

	if r[0][1] != token {
		t.Fatalf("wrong token: %s", r[0][1])
	}
}
