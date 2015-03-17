package id3

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
)

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
	d.r = io.LimitReader(d.r, int64(header.Size))

	return header, nil
}

func (d *Decoder) remaining() int64 {
	return d.r.(*io.LimitedReader).N
}

// Parse parses a tag.
//
// Parse will always return a valid tag. In the case of an error, the
// tag will be empty.
func (d *Decoder) Parse() (*Tag, error) {
	// TODO return how many bytes we read into the reader; so people
	// know where the audio begins
	tag := NewTag()
	header, err := d.ParseHeader()
	if err != nil {
		return tag, err
	}
	tag.Header = header

	// FIXME consider moving this to ParseHeader
	if header.Flags.ExtendedHeader() {
		panic("not implemented: cannot parse extended header")
	}

	if header.Flags.Unsynchronisation() {
		panic("not implemented: cannot parse unsynchronised tag")
	}

	for {
		frame, err := d.ParseFrame()
		if err != nil {
			if err == io.EOF {
				break
			}

			return tag, err
		}
		tag.Frames[frame.ID()] = append(tag.Frames[frame.ID()], frame)
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
	var bytes struct {
		Magic   [3]byte
		Version [2]byte
		Flags   byte
		Size    [4]byte
	}

	err = binary.Read(d.r, binary.BigEndian, &bytes)
	if err != nil {
		return header, err
	}
	if bytes.Magic != [3]byte{0x49, 0x44, 0x33} {
		return TagHeader{}, notATagHeader{bytes.Magic}
	}
	version := Version(int16(bytes.Version[0])<<8 | int16(bytes.Version[1]))
	if bytes.Version[0] > 4 || bytes.Version[0] < 3 {
		return TagHeader{}, UnsupportedVersion{version}
	}

	header.Version = version
	header.Flags = HeaderFlags(bytes.Flags)
	header.Size = desynchsafeInt(bytes.Size)

	return header, nil
}

// ParseFrame reads the next ID3 frame. When it reaches padding, it
// will discard it and return io.EOF. This should set the reader
// immediately before the audio data.
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

		return nil, NotAFrameHeader{headerBytes}
	}

	header.id = FrameType(headerBytes.ID[:])
	header.flags = FrameFlags(int16(headerBytes.Flags[0])<<8 | int16(headerBytes.Flags[1]))
	frameSize := desynchsafeInt(headerBytes.Size)

	if header.flags.Compressed() {
		panic("not implemented: cannot read compressed frame")
		// TODO: Read decompressed size (4 bytes)
	}

	if header.flags.Encrypted() {
		panic("not implemented: cannot read encrypted frame")
		// TODO: Read encryption method (1 byte)
	}

	if header.flags.Grouped() {
		panic("not implemented: cannot read grouped frame")
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

	switch header.id {
	case "TXXX":
		return d.readTXXXFrame(header, frameSize)
	case "WXXX":
		return d.readWXXXFrame(header, frameSize)
	case "UFID":
		return d.readUFIDFrame(header, frameSize)
	case "COMM":
		return d.readCOMMFrame(header, frameSize)
	case "PRIV":
		return d.readPRIVFrame(header, frameSize)
	case "APIC":
		return d.readAPICFrame(header, frameSize)
	case "MCDI":
		return d.readMCDIFrame(header, frameSize)
	case "USLT":
		return d.readUFIDFrame(header, frameSize)
	default:
		data := make([]byte, frameSize)
		n, err := d.r.Read(data)

		return UnsupportedFrame{
			FrameHeader: header,
			Data:        data[:n],
		}, err
	}
}

// New creates a new file from an existing *os.File and Tag. If you
// plan to save tags the file needs to be opened read and write.
func NewFile(file *os.File, tag *Tag) (*File, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	f := &File{
		f:        file,
		fileSize: stat.Size(),
		Tag:      tag,
	}

	f.audioReader = io.NewSectionReader(file, tagHeaderSize+int64(tag.Header.Size), f.fileSize-int64(tag.Header.Size))

	return f, nil
}

// Open opens the file with the given name in RW mode and parses its
// tag. If there is no tag, (*File).HasTag() will return false.
//
// Call Close() to close the underlying *os.File when done.
func Open(name string) (*File, error) {
	// TODO improve documentation. HasTag() will only be false until
	// the first save; and there will be an empty tag to work with.
	f, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	d := NewDecoder(f)
	tag, err := d.Parse()
	if err != nil {
		if _, ok := err.(notATagHeader); !ok {
			return nil, err
		}
	}
	file, err := NewFile(f, tag)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// HasTag returns true when the underlying file has a tag.
func (f *File) HasTag() bool {
	return f.Tag.Header.Version > 0
}

func (d *Decoder) readTXXXFrame(header FrameHeader, frameSize int) (Frame, error) {
	var encoding Encoding
	frame := UserTextInformationFrame{FrameHeader: header}
	rest := make([]byte, frameSize-1)

	err := readBinary(d.r, &encoding, &rest)
	if err != nil {
		return nil, err
	}
	parts := splitNullN(rest, encoding, 2)

	frame.Description = string(encoding.toUTF8(parts[0]))
	frame.Text = string(encoding.toUTF8(parts[1]))

	return frame, nil
}

func (d *Decoder) readWXXXFrame(header FrameHeader, frameSize int) (Frame, error) {
	var encoding Encoding
	frame := UserDefinedURLLinkFrame{FrameHeader: header}
	rest := make([]byte, frameSize-1)

	err := readBinary(d.r, &encoding, &rest)
	if err != nil {
		return nil, err
	}

	parts := splitNullN(rest, encoding, 2)
	frame.Description = string(encoding.toUTF8(parts[0]))
	frame.URL = string(iso88591ToUTF8(parts[1]))

	return frame, nil
}

func (d *Decoder) readUFIDFrame(header FrameHeader, frameSize int) (Frame, error) {
	frame := UniqueFileIdentifierFrame{FrameHeader: header}
	rest := make([]byte, frameSize)

	err := binary.Read(d.r, binary.BigEndian, rest)
	if err != nil {
		return nil, err
	}

	parts := bytes.SplitN(rest, []byte{0}, 2)
	frame.Owner = string(iso88591.toUTF8(parts[0]))
	frame.Identifier = parts[1]

	return frame, nil
}

func (d *Decoder) readCOMMFrame(header FrameHeader, frameSize int) (Frame, error) {
	frame := CommentFrame{FrameHeader: header}
	var (
		encoding Encoding
		language [3]byte
		rest     []byte
	)
	rest = make([]byte, frameSize-4)

	err := readBinary(d.r, &encoding, &language, &rest)
	if err != nil {
		return nil, err
	}

	parts := splitNullN(rest, encoding, 2)

	frame.Language = string(language[:])

	frame.Description = string(encoding.toUTF8(parts[0]))
	frame.Text = string(encoding.toUTF8(parts[1]))

	return frame, nil
}

func (d *Decoder) readPRIVFrame(header FrameHeader, frameSize int) (Frame, error) {
	frame := PrivateFrame{FrameHeader: header}
	data := make([]byte, frameSize)
	_, err := d.r.Read(data)
	if err != nil {
		return frame, err
	}

	parts := bytes.SplitN(data, nul, 2)
	frame.Owner = parts[0]
	frame.Data = parts[1]

	return frame, nil
}

func (d *Decoder) readAPICFrame(header FrameHeader, frameSize int) (Frame, error) {
	frame := PictureFrame{FrameHeader: header}
	var (
		encoding Encoding
		rest     []byte
	)
	rest = make([]byte, frameSize-1)
	err := readBinary(d.r, &encoding, &rest)
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

func (d *Decoder) readMCDIFrame(header FrameHeader, frameSize int) (Frame, error) {
	frame := MusicCDIdentifierFrame{FrameHeader: header}
	frame.TOC = make([]byte, frameSize)
	_, err := d.r.Read(frame.TOC)
	return frame, err
}

func (d *Decoder) readUSLTFrame(header FrameHeader, frameSize int) (Frame, error) {
	frame := UnsynchronisedLyricsFrame{FrameHeader: header}
	var (
		encoding Encoding
		language [3]byte
		rest     []byte
	)
	rest = make([]byte, frameSize-4)

	err := readBinary(d.r, &encoding, &language, rest)
	if err != nil {
		return frame, err
	}

	parts := splitNullN(rest, encoding, 2)
	frame.Language = string(language[:])

	frame.Description = string(encoding.toUTF8(parts[0]))
	frame.Lyrics = string(encoding.toUTF8(parts[1]))

	return frame, nil
}
