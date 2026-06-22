package pipeline

import (
	"encoding/json"
	"testing"
)

func TestStepEventKeepsEmptyTextFields(t *testing.T) {
	raw, err := json.Marshal(StepEvent{
		Chain:   "hearing",
		Step:    StepASR,
		SrcText: "",
		IsFinal: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["srcText"]; !ok {
		t.Fatalf("srcText field omitted from %s", raw)
	}
	if _, ok := got["dstText"]; !ok {
		t.Fatalf("dstText field omitted from %s", raw)
	}
}
