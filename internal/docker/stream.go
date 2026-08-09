package docker

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const maxLogPayload = 8 * 1024 * 1024

func streamDockerFrames(body io.Reader, onFrame func(Frame) error) error {
	reader := bufio.NewReaderSize(body, 32*1024)
	peek, err := reader.Peek(8)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return err
	}
	if len(peek) < 8 || !isDockerMultiplexed(peek) {
		return streamRawDockerFrames(reader, onFrame)
	}

	for {
		header := make([]byte, 8)
		_, err := io.ReadFull(reader, header)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return err
		}
		length := int(binary.BigEndian.Uint32(header[4:]))
		if length < 0 || length > maxLogPayload {
			return fmt.Errorf("docker follow frame is too large (%d bytes)", length)
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		stream := "stdout"
		if header[0] == 2 {
			stream = "stderr"
		}
		if err := onFrame(Frame{Stream: stream, Data: payload}); err != nil {
			return err
		}
	}
}

func isDockerMultiplexed(header []byte) bool {
	return len(header) >= 8 && (header[0] == 1 || header[0] == 2) && header[1] == 0 && header[2] == 0 && header[3] == 0
}

func streamRawDockerFrames(reader *bufio.Reader, onFrame func(Frame) error) error {
	line := make([]byte, 0, 256)
	for {
		chunk, err := reader.ReadSlice('\n')
		if len(chunk) > 0 {
			if len(line)+len(chunk) > maxLogPayload {
				return fmt.Errorf("docker follow line is too large")
			}
			line = append(line, chunk...)
			if chunk[len(chunk)-1] == '\n' {
				if callbackErr := onFrame(Frame{Stream: "stdout", Data: line}); callbackErr != nil {
					return callbackErr
				}
				line = line[:0]
			}
		}
		if err == nil || errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(line) > 0 {
				return onFrame(Frame{Stream: "stdout", Data: line})
			}
			return nil
		}
		return err
	}
}

func readDockerFrames(body io.Reader) ([]Frame, error) {
	reader := bufio.NewReaderSize(body, 32*1024)
	peek, err := reader.Peek(8)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(peek) < 8 || !isDockerMultiplexed(peek) {
		payload, readErr := io.ReadAll(io.LimitReader(reader, maxLogPayload+1))
		if len(payload) > maxLogPayload {
			return nil, fmt.Errorf("docker log payload is too large")
		}
		return []Frame{{Stream: "stdout", Data: payload}}, readErr
	}

	frames := make([]Frame, 0, 8)
	var total int
	for {
		header := make([]byte, 8)
		_, err := io.ReadFull(reader, header)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return frames, err
		}
		length := int(binary.BigEndian.Uint32(header[4:]))
		if length < 0 || length > maxLogPayload || total+length > maxLogPayload {
			return frames, fmt.Errorf("docker log frame is too large (%d bytes)", length)
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return frames, err
		}
		stream := "stdout"
		if header[0] == 2 {
			stream = "stderr"
		}
		frames = append(frames, Frame{Stream: stream, Data: payload})
		total += length
	}
	return frames, nil
}
