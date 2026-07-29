package upstream

import "testing"

func TestParseOpenAIModels(t *testing.T) {
	body := []byte(`{"object":"list","data":[{"id":"b-model"},{"id":"a-model"},{"id":"a-model"}]}`)
	ids, err := parseModelIDs(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "a-model" || ids[1] != "b-model" {
		t.Fatalf("got %#v", ids)
	}
}
