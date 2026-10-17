package head

// SoundDir is the voice-direction source: the ReSpeaker 2-Mic HAT on the head
// (Mic: arecord -c 2 -r 16000, GCC-PHAT between the capsules) sends one event per
// utterance. The head forwards it to the body as "sound_dir"; the body adds the
// head angle and it lands in listen. NoSoundDir is used without the HAT.
type SoundDir interface {
	Name() string
	Events() <-chan SoundEvent // nil = no events
}

// SoundEvent: angle relative to the HEAD front (the mics are on the head), + right,
// −90…90 (two mics cannot tell front from back). ID matches the audio stream of the
// same utterance. Text is optional.
type SoundEvent struct {
	AngleDeg float64 `json:"angle_deg"`
	LevelDB  float64 `json:"level_db"`
	ID       uint64  `json:"id,omitempty"`
	Text     string  `json:"text,omitempty"`
}

// AudioSource streams the voiced audio (Mic implements it).
type AudioSource interface {
	Audio() <-chan AudioChunk
}

type NoSoundDir struct{}

func (NoSoundDir) Name() string              { return "none" }
func (NoSoundDir) Events() <-chan SoundEvent { return nil }
