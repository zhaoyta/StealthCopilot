package video

// Frame is a BGRA video frame with a millisecond presentation timestamp.
type Frame struct {
	Data []byte
	PTS  int64
}
