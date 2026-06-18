package audio

import "testing"

func TestParseAudioToolboxOutputIndex(t *testing.T) {
	raw := `[AudioToolbox @ 0x1517060f0] CoreAudio devices:
[AudioToolbox @ 0x1517060f0] [0]                  BlackHole 2ch, BlackHole2ch_UID
[AudioToolbox @ 0x1517060f0] [1]                         (null), BuiltInHeadphoneOutputDevice
[AudioToolbox @ 0x1517060f0] [7]             iFlyrecAudioDevice, iFlyrecAudioDevice2ch_UID`

	idx, ok := parseAudioToolboxOutputIndex(raw, "BlackHole 2ch")
	if !ok {
		t.Fatal("expected BlackHole output index")
	}
	if idx != 0 {
		t.Fatalf("index = %d, want 0", idx)
	}
}

func TestParseAudioToolboxOutputIndex_NotFound(t *testing.T) {
	if _, ok := parseAudioToolboxOutputIndex(`[AudioToolbox] [0] BlackHole 2ch, BlackHole2ch_UID`, "Missing"); ok {
		t.Fatal("unexpected match")
	}
}
