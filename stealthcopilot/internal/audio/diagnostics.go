package audio

import "encoding/binary"

func PCMPeak(frame []byte) int {
	peak := 0
	for i := 0; i+1 < len(frame); i += 2 {
		v := int(int16(binary.LittleEndian.Uint16(frame[i:])))
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
	}
	return peak
}

func pcmPeak(frame []byte) int {
	return PCMPeak(frame)
}
