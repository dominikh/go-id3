package id3

import (
	"io"
	"time"
)

type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) WriteHeader(size int) error {
	h := generateHeader(size)
	_, err := e.w.Write(h)
	return err
}

func (e *Encoder) WriteFrame(f Frame) error {
	b := f.Header().serialize(f.size() - frameLength)
	_, err := e.w.Write(b)
	if err != nil {
		return err
	}
	b = f.Encode()
	_, err = e.w.Write(b)
	return err
}

func (t *Tag) Encode(w io.Writer) error {
	t.SetTextFrameTime("TDTG", time.Now().UTC())
	enc := NewEncoder(w)
	err := enc.WriteHeader(t.Frames.size() + Padding)
	if err != nil {
		return err
	}

	// TODO write important frames first
	for _, frames := range t.Frames {
		for _, frame := range frames {
			err := enc.WriteFrame(frame)
			if err != nil {
				return err
			}
		}
	}

	_, err = w.Write(make([]byte, Padding))
	return err
}
