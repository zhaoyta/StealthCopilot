package system

import "testing"

func TestParseMacAudioOutputs_UsesNumericIDs(t *testing.T) {
	raw := []byte(`{
		"SPAudioDataType": [
			{
				"_name": "Audio",
				"_items": [
					{
						"_name": "External Headphones",
						"spaudio_output_source": "External Headphones"
					},
					{
						"_name": "Mac mini Speakers",
						"spaudio_output_source": "Mac mini Speakers"
					}
				]
			}
		]
	}`)

	outputs := parseMacAudioOutputs(raw)
	if len(outputs) != 2 {
		t.Fatalf("len(outputs) = %d, want 2", len(outputs))
	}
	if outputs[0].ID != "0" || outputs[0].Name != "External Headphones" {
		t.Fatalf("outputs[0] = %#v, want numeric ID 0 with friendly name", outputs[0])
	}
	if outputs[1].ID != "1" || outputs[1].Name != "Mac mini Speakers" {
		t.Fatalf("outputs[1] = %#v, want numeric ID 1 with friendly name", outputs[1])
	}
}
