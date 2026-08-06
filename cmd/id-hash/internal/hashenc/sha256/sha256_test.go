package sha256

import "testing"

func TestSHA256_GenerateResult(t *testing.T) {
	hf := New([]byte("5AVluLb5L0l17J+u9;Zc*CB%S33&?o.o#1m/I1(6^x<_:*/Eo!JJbmqPvJ)XXI6+"))

	b, err := hf.GenerateResult([]byte("380672241715"))
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != "l6HgDwn4p/2njJdYLH+EHrTxNsrC3Oh6ob01XsQrCsA=" {
		t.Fatal("wrong result: ", string(b))
	}
}
