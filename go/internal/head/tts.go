package head

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// wyomingTTS asks a Wyoming text-to-speech server (wyoming-piper on the bb8-voice
// server) for text and returns a WAV with the samples scaled by vol. The head
// dials out, as it does for the body link; it still opens no ports.
func wyomingTTS(ctx context.Context, addr, text string, vol float64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	}
	req, _ := json.Marshal(map[string]any{"type": "synthesize", "data": map[string]any{"text": text}})
	if _, err := c.Write(append(req, '\n')); err != nil {
		return nil, err
	}
	r := bufio.NewReader(c)
	var pcm bytes.Buffer
	rate, width, channels := 22050, 2, 1
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("wyoming: %w", err)
		}
		var ev struct {
			Type          string          `json:"type"`
			Data          json.RawMessage `json:"data"`
			DataLength    int             `json:"data_length"`
			PayloadLength int             `json:"payload_length"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("wyoming: %w", err)
		}
		if ev.DataLength > 0 {
			ev.Data = make([]byte, ev.DataLength)
			if _, err := io.ReadFull(r, ev.Data); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, ev.PayloadLength)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
		switch ev.Type {
		case "audio-start":
			var f struct{ Rate, Width, Channels int }
			if json.Unmarshal(ev.Data, &f) == nil && f.Rate > 0 {
				rate, width, channels = f.Rate, f.Width, f.Channels
			}
		case "audio-chunk":
			pcm.Write(payload)
		case "audio-stop":
			if width != 2 || pcm.Len() == 0 {
				return nil, errors.New("wyoming: no 16-bit audio")
			}
			return wavFrom(scale(pcm.Bytes(), vol), rate, channels), nil
		case "error":
			return nil, fmt.Errorf("wyoming: %s", ev.Data)
		}
	}
}

func scale(pcm []byte, vol float64) []byte {
	if vol >= 0.999 {
		return pcm
	}
	for i := 0; i+1 < len(pcm); i += 2 {
		s := float64(int16(binary.LittleEndian.Uint16(pcm[i:]))) * vol
		binary.LittleEndian.PutUint16(pcm[i:], uint16(int16(s)))
	}
	return pcm
}

func wavFrom(pcm []byte, rate, channels int) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(channels), uint32(rate),
		uint32(rate * channels * 2), uint16(channels * 2), uint16(16)} {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}
