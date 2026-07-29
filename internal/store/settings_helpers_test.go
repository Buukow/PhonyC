package store

import "testing"

func TestParseStatusCodeList(t *testing.T) {
	got := ParseStatusCodeList("401, 403,404,503", nil)
	if len(got) != 4 || got[0] != 401 || got[3] != 503 {
		t.Fatalf("%v", got)
	}
	if !StatusCodeListContains(got, 403) || StatusCodeListContains(got, 500) {
		t.Fatal("contains")
	}
}

func TestChannelRoutable(t *testing.T) {
	c := Channel{Enabled: true, TempDisabled: false}
	if !c.Routable() {
		t.Fatal("should routable")
	}
	c.TempDisabled = true
	if c.Routable() {
		t.Fatal("temp disabled not routable")
	}
	c.Enabled = false
	c.TempDisabled = false
	if c.Routable() {
		t.Fatal("disabled not routable")
	}
}
