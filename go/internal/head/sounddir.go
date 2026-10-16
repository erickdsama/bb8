package head

// SoundDir is the voice-direction hook for the ReSpeaker 2-Mic HAT. An
// implementation reads the two channels (arecord -c 2 -r 16000), estimates the
// arrival angle (GCC-PHAT between the mics, 58 mm apart) and the level, and
// sends one event per utterance. The head forwards each event to the body as
// "sound_dir", where it lands in listen (same as POST /hearing).
//
// Pending hardware: NoSoundDir is used until the HAT is installed.
type SoundDir interface {
	Name() string
	Events() <-chan SoundEvent // nil = no events
}

// SoundEvent: angle relative to the body front, + right, −90…90 (two mics cannot
// tell front from back). Text is optional (filled if a transcriber runs here).
type SoundEvent struct {
	AngleDeg float64 `json:"angle_deg"`
	LevelDB  float64 `json:"level_db"`
	Text     string  `json:"text,omitempty"`
}

type NoSoundDir struct{}

func (NoSoundDir) Name() string              { return "none" }
func (NoSoundDir) Events() <-chan SoundEvent { return nil }
