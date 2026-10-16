package body

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/erickdsama/bb8/go/internal/head"
	"github.com/erickdsama/bb8/go/internal/link"
)

type fakeMic struct {
	ev    chan head.SoundEvent
	audio chan head.AudioChunk
}

func (f *fakeMic) Name() string                   { return "fake" }
func (f *fakeMic) Events() <-chan head.SoundEvent { return f.ev }
func (f *fakeMic) Audio() <-chan head.AudioChunk  { return f.audio }

type micBackend struct {
	*head.Sim
	m *fakeMic
}

func (b micBackend) Sound() head.SoundDir { return b.m }

// Head audio and sound direction go over the link: the hub gets the frames and
// OnSound gets the head angle the body stamped at audio_start.
func TestHeadAudioOverLink(t *testing.T) {
	dir := t.TempDir()
	if err := link.Pair(dir, nil); err != nil {
		t.Fatal(err)
	}
	scfg, _ := link.ServerTLS(dir)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", scfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := &link.Server{}
	h := NewHead(srv)
	h.HeadDeg = func() float64 { return 30 }
	got := make(chan [3]float64, 1)
	h.OnSound = func(_ *string, a, l, hd *float64) { got <- [3]float64{*a, *l, *hd} }
	frames, stop := h.Audio.Subscribe()
	defer stop()
	go srv.Serve(ctx, ln)

	m := &fakeMic{ev: make(chan head.SoundEvent, 1), audio: make(chan head.AudioChunk, 4)}
	ccfg, _ := link.ClientTLS(dir)
	hd := &head.Head{B: micBackend{Sim: head.NewSim("http://127.0.0.1:1"), m: m}}
	go hd.Run(ctx, ln.Addr().String(), ccfg)

	m.audio <- head.AudioChunk{Kind: "start", ID: 7}
	m.audio <- head.AudioChunk{Kind: "data", ID: 7, PCM: []byte{1, 2, 3, 4}}
	m.audio <- head.AudioChunk{Kind: "end", ID: 7}
	var types []string
	for len(types) < 3 {
		select {
		case f := <-frames:
			types = append(types, f.Type)
			if f.Type == "audio" && len(f.PCM) != 4 {
				t.Fatal("pcm", f.PCM)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("frames so far:", types)
		}
	}
	if types[0] != "audio_start" || types[1] != "audio" || types[2] != "audio_end" {
		t.Fatal(types)
	}
	m.ev <- head.SoundEvent{AngleDeg: -20, LevelDB: 61, ID: 7}
	select {
	case v := <-got:
		if v != [3]float64{-20, 61, 30} {
			t.Fatal(v)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no sound_dir")
	}
}
