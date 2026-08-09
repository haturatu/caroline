package docker

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func dockerTestFrame(stream byte, payload string) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = stream
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}

func TestReadDockerFrames(t *testing.T) {
	input := append(dockerTestFrame(1, "stdout line\n"), dockerTestFrame(2, "stderr line\n")...)
	frames, err := readDockerFrames(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("readDockerFrames returned error: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}
	if frames[0].Stream != "stdout" || string(frames[0].Data) != "stdout line\n" {
		t.Fatalf("unexpected stdout frame: %#v", frames[0])
	}
	if frames[1].Stream != "stderr" || string(frames[1].Data) != "stderr line\n" {
		t.Fatalf("unexpected stderr frame: %#v", frames[1])
	}

	raw, err := readDockerFrames(strings.NewReader("raw tty log\n"))
	if err != nil || len(raw) != 1 || raw[0].Stream != "stdout" || string(raw[0].Data) != "raw tty log\n" {
		t.Fatalf("unexpected raw frame: %#v, error: %v", raw, err)
	}
}

func TestStreamDockerFrames(t *testing.T) {
	input := append(dockerTestFrame(2, "live error\n"), dockerTestFrame(1, "live info\n")...)
	var got []Frame
	if err := streamDockerFrames(bytes.NewReader(input), func(frame Frame) error {
		got = append(got, frame)
		return nil
	}); err != nil {
		t.Fatalf("streamDockerFrames returned error: %v", err)
	}
	if len(got) != 2 || got[0].Stream != "stderr" || got[1].Stream != "stdout" {
		t.Fatalf("unexpected streamed frames: %#v", got)
	}
}

func TestReadDockerFramesRejectsOversizedRawPayload(t *testing.T) {
	_, err := readDockerFrames(bytes.NewReader(bytes.Repeat([]byte{'x'}, maxLogPayload+1)))
	if err == nil {
		t.Fatal("readDockerFrames accepted an oversized raw payload")
	}
}
