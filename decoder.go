package id3

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
)

type fnFrameReader func(r io.Reader, header FrameHeader, frameSize int) (Frame, error)

var frameReaders = map[FrameType]fnFrameReader{
	"TXXX": readTXXXFrame,
	"WXXX": readWXXXFrame,
	"UFID": readUFIDFrame,
	"COMM": readCOMMFrame,
	"PRIV": readPRIVFrame,
	"APIC": readAPICFrame,
	"MCDI": readMCDIFrame,
	"USLT": readUSLTFrame,
}

// TODO support the following frames:
// - AENC - Audio encryption
// - ASPI - Audio seek point index
// - COMR - Commercial frame
// - ENCR - Encryption method registration
// - EQU2 - Equalisation (2)
// - ETCO - Event timing codes
// - GEOB - General encapsulated object
// - GRID - Group identification registration
// - LINK - Linked information
// - MLLT - MPEG location lookup table
// - OWNE - Ownership frame
// - PCNT - Play counter
// - POPM - Popularimeter
// - POSS - Position synchronisation frame
// - RBUF - Recommended buffer size
// - RVA2 - Relative volume adjustment (2)
// - RVRB - Reverb
// - SEEK - Seek frame
// - SIGN -
// - SYLT - Synchronised lyric/text
// - SYTC - Synchronised tempo codes

type Peeker interface {
	Peek(n int) ([]byte, error)
}

// Check reports whether r looks like it starts with an ID3 tag.
func Check(r Peeker) (bool, error) {
	b, err := r.Peek(3)
	if err != nil {
		return false, err
	}
	return bytes.Equal(b, Magic), nil
}

type Decoder struct {
	r io.Reader
	h TagHeader
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// ParseHeader parses only the ID3 header.
func (d *Decoder) ParseHeader() (TagHeader, error) {
	header, err := d.readHeader()
	if err != nil {
		return TagHeader{}, err
	}

	d.h = header
	d.r = io.LimitReader(d.r, int64(header.size))

	return header, nil
}

func (d *Decoder) remaining() int64 {
	return d.r.(*io.LimitedReader).N
}

type UnimplementedFeatureError struct {
	Feature string
}

func (err UnimplementedFeatureError) Error() string {
	return "unsupported feature: " + err.Feature
}

// Parse parses a tag.
//
// Parse will always return a valid tag. In the case of an error, the
// tag will be empty.
//
// If Parse successfully parsed a tag, the reader will be positioned
// immediately after the tag, which usually is directly before audio
// data. If there wasn't a valid tag, however, the position of the
// reader is undefined. If you're not sure if your reader starts with
// a tag at all, consider using Check first.
//
// Parse cannot be called if either ParseHeader or ParseFrame have
// been called for the current tag.
func (d *Decoder) Parse() (*Tag, error) {
	tag := NewTag()
	header, err := d.ParseHeader()
	if err != nil {
		return tag, err
	}
	tag.Header = header

	// FIXME consider moving this to ParseHeader
	if header.Flags.ExtendedHeader() {
		return tag, UnimplementedFeatureError{"extended header"}
	}

	if header.Flags.Unsynchronisation() {
		return tag, UnimplementedFeatureError{"unsynchronised tag"}
	}

	for {
		frame, err := d.ParseFrame()
		if err != nil {
			if err == io.EOF {
				break
			}

			return tag, err
		}
		tag.Frames = append(tag.Frames, frame)
	}

	if header.Version < 0x0400 {
		tag.upgrade()
	}

	return tag, nil
}

func readBinary(r io.Reader, args ...interface{}) (err error) {
	for _, arg := range args {
		err = binary.Read(r, binary.BigEndian, arg)
		if err != nil {
			break
		}
	}

	return
}

// readHeader reads an ID3v2 header. It expects the reader to be
// seeked to the beginning of the header.
func (d *Decoder) readHeader() (header TagHeader, err error) {
	var data struct {
		Magic   [3]byte
		Version [2]byte
		Flags   byte
		Size    [4]byte
	}

	err = binary.Read(d.r, binary.BigEndian, &data)
	if err != nil {
		return header, err
	}
	if !bytes.Equal(data.Magic[:], Magic) {
		return TagHeader{}, InvalidTagHeaderError{data.Magic[:]}
	}
	version := Version(int16(data.Version[0])<<8 | int16(data.Version[1]))
	if data.Version[0] > 4 || data.Version[0] < 3 {
		return TagHeader{}, UnsupportedVersionError{version}
	}

	header.Version = version
	header.Flags = HeaderFlags(data.Flags)
	header.size = desynchsafeInt(data.Size)

	return header, nil
}

// ParseFrame reads the next ID3 frame. When it reaches padding, it
// will read and discard all of it and return io.EOF. This should set
// the reader immediately before the audio data.
//
// ParseHeader must be called before calling ParseFrame.
func (d *Decoder) ParseFrame() (Frame, error) {
	if d.remaining() == 0 {
		return nil, io.EOF
	}

	var (
		headerBytes struct {
			ID    [4]byte
			Size  [4]byte
			Flags [2]byte
		}
		header FrameHeader
	)

	err := binary.Read(d.r, binary.BigEndian, &headerBytes)
	if err != nil {
		return nil, err
	}

	// We're in the padding, discard remaining bytes and return io.EOF
	if headerBytes.ID == [4]byte{0, 0, 0, 0} {
		_, err := io.Copy(ioutil.Discard, d.r)
		if err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	for _, byte := range headerBytes.ID {
		// Allow 0-9
		if byte >= 48 && byte <= 57 {
			continue
		}

		// Allow A-Z
		if byte >= 65 && byte <= 90 {
			continue
		}

		return nil, InvalidFrameHeaderError{headerBytes}
	}

	header.id = FrameType(headerBytes.ID[:])
	header.flags = FrameFlags(int16(headerBytes.Flags[0])<<8 | int16(headerBytes.Flags[1]))
	frameSize := desynchsafeInt(headerBytes.Size)

	if header.flags.Compressed() {
		return nil, UnimplementedFeatureError{"compressed frame"}
		// TODO: Read decompressed size (4 bytes)
	}

	if header.flags.Encrypted() {
		return nil, UnimplementedFeatureError{"encrypted frame"}
		// TODO: Read encryption method (1 byte)
	}

	if header.flags.Grouped() {
		return nil, UnimplementedFeatureError{"grouped frame"}
		// TODO: Read group identifier (1 byte)
	}

	if header.id[0] == 'T' && header.id != "TXXX" {
		var encoding Encoding
		frame := TextInformationFrame{FrameHeader: header}
		information := make([]byte, frameSize-1)
		err := readBinary(d.r, &encoding, &information)
		if err != nil {
			return nil, err
		}

		frame.Text = string(encoding.toUTF8(information))

		return frame, nil
	}

	if header.id[0] == 'W' && header.id != "WXXX" {
		frame := URLLinkFrame{FrameHeader: header}
		url := make([]byte, frameSize)
		_, err = d.r.Read(url)
		if err != nil {
			return nil, err
		}
		frame.URL = string(iso88591.toUTF8(url))

		return frame, nil
	}

	fn, ok := frameReaders[header.id]
	if !ok {
		data := make([]byte, frameSize)
		n, err := d.r.Read(data)

		return UnsupportedFrame{
			FrameHeader: header,
			Data:        data[:n],
		}, err
	}
	return fn(d.r, header, frameSize)
}

func readTXXXFrame(r io.Reader, header FrameHeader, frameSize int) (Frame, error) {
	var encoding Encoding
	frame := UserTextInformationFrame{FrameHeader: header}
	rest := make([]byte, frameSize-1)

	err := readBinary(r, &encoding, &rest)
	if err != nil {
		return nil, err
	}
	parts := splitNullN(rest, encoding, 2)

	frame.Description = string(encoding.toUTF8(parts[0]))
	frame.Text = string(encoding.toUTF8(parts[1]))

	return frame, nil
}

func readWXXXFrame(r io.Reader, header FrameHeader, frameSize int) (Frame, error) {
	var encoding Encoding
	frame := UserDefinedURLLinkFrame{FrameHeader: header}
	rest := make([]byte, frameSize-1)

	err := readBinary(r, &encoding, &rest)
	if err != nil {
		return nil, err
	}

	parts := splitNullN(rest, encoding, 2)
	frame.Description = string(encoding.toUTF8(parts[0]))
	frame.URL = string(iso88591ToUTF8(parts[1]))

	return frame, nil
}

func readUFIDFrame(r io.Reader, header FrameHeader, frameSize int) (Frame, error) {
	frame := UniqueFileIdentifierFrame{FrameHeader: header}
	rest := make([]byte, frameSize)

	err := binary.Read(r, binary.BigEndian, rest)
	if err != nil {
		return nil, err
	}

	parts := bytes.SplitN(rest, []byte{0}, 2)
	frame.Owner = string(iso88591.toUTF8(parts[0]))
	frame.Identifier = parts[1]

	return frame, nil
}

func readCOMMFrame(r io.Reader, header FrameHeader, frameSize int) (Frame, error) {
	frame := CommentFrame{FrameHeader: header}
	var (
		encoding Encoding
		language [3]byte
		rest     []byte
	)
	rest = make([]byte, frameSize-4)

	err := readBinary(r, &encoding, &language, &rest)
	if err != nil {
		return nil, err
	}

	parts := splitNullN(rest, encoding, 2)

	frame.Language = string(language[:])

	frame.Description = string(encoding.toUTF8(parts[0]))
	frame.Text = string(encoding.toUTF8(parts[1]))

	return frame, nil
}

func readPRIVFrame(r io.Reader, header FrameHeader, frameSize int) (Frame, error) {
	frame := PrivateFrame{FrameHeader: header}
	data := make([]byte, frameSize)
	_, err := r.Read(data)
	if err != nil {
		return frame, err
	}

	parts := bytes.SplitN(data, nul, 2)
	frame.Owner = parts[0]
	frame.Data = parts[1]

	return frame, nil
}

func readAPICFrame(r io.Reader, header FrameHeader, frameSize int) (Frame, error) {
	frame := PictureFrame{FrameHeader: header}
	var (
		encoding Encoding
		rest     []byte
	)
	rest = make([]byte, frameSize-1)
	err := readBinary(r, &encoding, &rest)
	if err != nil {
		return frame, err
	}

	parts1 := bytes.SplitN(rest, nul, 2)
	parts2 := splitNullN(parts1[1][1:], encoding, 2)

	frame.MIMEType = string(iso88591.toUTF8(parts1[0]))
	frame.PictureType = PictureType(parts1[1][0])
	frame.Description = string(encoding.toUTF8(parts2[0]))
	frame.Data = parts2[1]

	return frame, nil
}

func readMCDIFrame(r io.Reader, header FrameHeader, frameSize int) (Frame, error) {
	frame := MusicCDIdentifierFrame{FrameHeader: header}
	frame.TOC = make([]byte, frameSize)
	_, err := r.Read(frame.TOC)
	return frame, err
}

func readUSLTFrame(r io.Reader, header FrameHeader, frameSize int) (Frame, error) {
	frame := UnsynchronisedLyricsFrame{FrameHeader: header}
	var (
		encoding Encoding
		language [3]byte
		rest     []byte
	)
	rest = make([]byte, frameSize-4)

	err := readBinary(r, &encoding, &language, rest)
	if err != nil {
		return frame, err
	}

	parts := splitNullN(rest, encoding, 2)
	frame.Language = string(language[:])

	frame.Description = string(encoding.toUTF8(parts[0]))
	frame.Lyrics = string(encoding.toUTF8(parts[1]))

	return frame, nil
}
