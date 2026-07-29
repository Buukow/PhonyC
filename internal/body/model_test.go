package body

import (
	"bytes"
	"testing"
)

func TestPeekAndRewrite(t *testing.T) {
	in := []byte(`{"foo":1,"model":"gpt-test","nested":{"model":"nope"},"bar":true}`)
	m, start, end, err := PeekTopModel(in)
	if err != nil {
		t.Fatal(err)
	}
	if m != "gpt-test" {
		t.Fatalf("model=%q", m)
	}
	if !bytes.Equal(in[start:end], []byte(`"gpt-test"`)) {
		t.Fatalf("span=%s", in[start:end])
	}
	out, err := RewriteTopModel(in, `up"stream`)
	if err != nil {
		t.Fatal(err)
	}
	m2, _, _, err := PeekTopModel(out)
	if err != nil {
		t.Fatal(err)
	}
	if m2 != `up"stream` {
		t.Fatalf("rewritten=%q body=%s", m2, out)
	}
	if bytes.Contains(out, []byte(`"nope"`)) == false {
		t.Fatal("nested model should remain")
	}
}

func TestPeekMissing(t *testing.T) {
	_, _, _, err := PeekTopModel([]byte(`{"a":1}`))
	if err != ErrModelMissing {
		t.Fatalf("err=%v", err)
	}
}

func TestPeekNestedOnly(t *testing.T) {
	_, _, _, err := PeekTopModel([]byte(`{"metadata":{"model":"x"}}`))
	if err != ErrModelMissing {
		t.Fatalf("err=%v", err)
	}
}
