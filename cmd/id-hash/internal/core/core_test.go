package core

import (
	"bufio"
	"flag"
	"github.com/google/uuid"
	"github.com/mygaru/id-hash/cmd/id-hash/internal/hashenc"
	"log"
	"os"
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

func TestHash(t *testing.T) {
	flag.Set("function", "sha256")
	flag.Set("key", "ORK7zw4ekuU5IssbBQTxwnkMg9vRBFz9A//AH0mpszLgaVoVLxxfRWTqx6d2XL6R")
	flag.Parse()

	log.Printf(os.Getwd())
	f, err := os.Open("../../../../bin/input.csv")
	if err != nil {
		t.Fatal(err)
	}

	out, err := os.Create("../../../../bin/output.csv")
	if err != nil {
		t.Fatal(err)
	}

	sc := bufio.NewScanner(f)
	h := hashenc.Get()

	for sc.Scan() {
		hash, err := h.GenerateResult(sc.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		out.Write(hash)
		out.WriteString("\n")
	}
}
