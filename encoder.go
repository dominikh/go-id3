package id3

import (
	"io"
	"time"
)

func generateHeader(size int) []byte {
	size = synchsafeInt(size)

	// TODO flags
	return concat(id3byte, versionByte, nul, intToBytes(size))
}

type Encoder struct {
	w io.Writer
	// The amount of padding that will be added after the last frame.
	Padding int
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:       w,
		Padding: 1024,
	}
}

func (e *Encoder) WriteHeader(size int) error {
	h := generateHeader(size)
	_, err := e.w.Write(h)
	return err
}

func (e *Encoder) WriteFrame(f Frame) error {
	b := f.Header().serialize(f.Size() - frameLength)
	_, err := e.w.Write(b)
	if err != nil {
		return err
	}
	b = f.Encode()
	_, err = e.w.Write(b)
	return err
}

func (e *Encoder) WritePadding() error {
	_, err := e.w.Write(make([]byte, e.Padding))
	return err
}

func (e *Encoder) WriteTag(t *Tag) error {
	t.SetTextFrameTime("TDTG", time.Now().UTC())
	var size int
	for _, frame := range t.Frames {
		size += frame.Size()
	}
	err := e.WriteHeader(size + e.Padding)
	if err != nil {
		return err
	}

	// TODO write important frames first
	for _, frame := range t.Frames {
		err := e.WriteFrame(frame)
		if err != nil {
			return err
		}
	}

	return e.WritePadding()
}
