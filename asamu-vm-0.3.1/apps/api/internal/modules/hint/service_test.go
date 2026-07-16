package hint

import (
	"encoding/json"
	"testing"
)

func TestParsePublishedHints(t *testing.T) {
	items, err := parse(json.RawMessage(`[{"title":"入口","content":"比较响应差异","cost":25,"sortOrder":0}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "入口" || items[0].Content != "比较响应差异" || items[0].Cost != 25 || items[0].Index != 0 {
		t.Fatalf("unexpected parsed hint: %#v", items)
	}
}

func TestParseEmptyHints(t *testing.T) {
	items, err := parse(json.RawMessage(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no hints, got %#v", items)
	}
}
