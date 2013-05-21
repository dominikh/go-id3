package id3

import (
	"bytes"
	_ "code.google.com/p/go-charset/data"
	"encoding/binary"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	utf16p "unicode/utf16"
)

var _ = spew.Dump

// The amount of padding that will be added after the last frame
var Padding = 1024
var Logging logFlag

type logFlag bool

func (l logFlag) Println(args ...interface{}) {
	if l {
		log.Println(args...)
	}
}

const (
	iso88591 = 0
	utf16bom = 1
	utf16be  = 2
	utf8     = 3

	frameLength = 10
)

var (
	utf16nul = []byte{0, 0}
	nul      = []byte{0}

	utf8byte = []byte{utf8}

	id3byte     = []byte("ID3")
	versionByte = []byte{4, 0}
)

const TimeFormat = "2006-01-02T15:04:05"

var timeFormats = []string{
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02T15",
	"2006-01-02",
	"2006-01",
	"2006",
}

// TODO: ID3v2 extended header
// TODO: unsynchronisation

type HeaderFlags byte
type FrameFlags uint16
type Version int16
type Encoding byte
type FrameType string
type FramesMap map[FrameType][]Frame

type NotATagHeader struct {
	Magic [3]byte
}

type UnsupportedVersion struct {
	Version Version
}

type TagHeader struct {
	Version Version
	Flags   HeaderFlags
	Size    int
}

type File struct {
	f           *os.File
	fileSize    int64
	tagReader   io.ReadSeeker
	audioReader io.ReadSeeker
	hasTags     bool
	Header      TagHeader
	Frames      FramesMap
}

func (f FrameType) String() string {
	v, ok := FrameNames[f]
	if ok {
		return v
	}

	return string(f)
}

func (e Encoding) String() string {
	switch e {
	case iso88591:
		return "ISO-8859-1"
	case utf16bom:
		return "UTF-16"
	case utf16be:
		return "UTF-16BE"
	case utf8:
		return "UTF-8"
	default:
		return fmt.Sprintf("Unknown encoding %d", byte(e))
	}
}

func (e Encoding) terminator() []byte {
	switch e {
	case utf16bom, utf16be:
		return utf16nul
	default:
		return nul
	}
}

// TODO: HeaderFlags.String()
// TODO: FrameFlags.String()

func (err NotATagHeader) Error() string {
	return fmt.Sprintf("Not an ID3v2 header: %v", err.Magic)
}

func (err UnsupportedVersion) Error() string {
	return fmt.Sprintf("Unsupported version: %s", err.Version)
}

func (f HeaderFlags) Unsynchronisation() bool {
	return (f & 128) > 0
}

func (f HeaderFlags) ExtendedHeader() bool {
	return (f & 64) > 0
}

func (f HeaderFlags) Experimental() bool {
	return (f & 32) > 0
}

func (f HeaderFlags) UndefinedSet() bool {
	return (f & 31) > 0
}

func (f FrameFlags) PreserveTagAlteration() bool {
	return (f & 0x4000) == 0
}

func (f FrameFlags) PreserveFileAlteration() bool {
	return (f & 0x2000) == 0
}

func (f FrameFlags) ReadOnly() bool {
	return (f & 0x1000) > 0
}

func (f FrameFlags) Compressed() bool {
	return (f & 128) > 0
}

func (f FrameFlags) Encrypted() bool {
	return (f & 64) > 0
}

func (f FrameFlags) Grouped() bool {
	return (f & 32) > 0
}

func (v Version) String() string {
	return fmt.Sprintf("ID3v2.%.1d.%.1d", v>>8, v&0xFF)
}

// readHeader reads an ID3v2 header. It expects the reader to be
// seeked to the beginning of the header.
func readHeader(r io.Reader) (header TagHeader, n int, err error) {
	var (
		bytes struct {
			Magic   [3]byte
			Version [2]byte
			Flags   byte
			Size    [4]byte
		}
	)

	binary.Read(r, binary.BigEndian, &bytes.Magic)
	if bytes.Magic != [3]byte{0x49, 0x44, 0x33} {
		return TagHeader{}, 3, NotATagHeader{bytes.Magic}
	}
	binary.Read(r, binary.BigEndian, &bytes.Version)
	version := Version(int16(bytes.Version[0])<<8 | int16(bytes.Version[1]))
	if bytes.Version[0] != 4 {
		return TagHeader{}, 5, UnsupportedVersion{version}
	}
	binary.Read(r, binary.BigEndian, &bytes.Flags)
	binary.Read(r, binary.BigEndian, &bytes.Size)

	header.Version = version
	header.Flags = HeaderFlags(bytes.Flags)
	header.Size = desynchsafeInt(bytes.Size)

	return header, 10, nil
}

func readFrame(r io.Reader) (Frame, error) {
	var (
		headerBytes struct {
			ID    [4]byte
			Size  [4]byte
			Flags [2]byte
		}
		header FrameHeader
	)

	binary.Read(r, binary.BigEndian, &headerBytes.ID)
	binary.Read(r, binary.BigEndian, &headerBytes.Size)
	binary.Read(r, binary.BigEndian, &headerBytes.Flags)

	header.id = FrameType(headerBytes.ID[:])
	header.flags = FrameFlags(int16(headerBytes.Flags[0])<<8 | int16(headerBytes.Flags[1]))
	headerSize := desynchsafeInt(headerBytes.Size)

	if header.flags.Compressed() {
		// TODO: Read decompressed size (4 bytes)
	}

	if header.flags.Encrypted() {
		// TODO: Read encryption method (1 byte)
	}

	if header.flags.Grouped() {
		// TODO: Read group identifier (1 byte)
	}

	// We're in the padding, return io.EOF
	if header.id[0] == 0 {
		return nil, io.EOF
	}

	if header.id[0] == 'T' && header.id != "TXXX" {
		var encoding Encoding
		frame := TextInformationFrame{FrameHeader: header}
		information := make([]byte, headerSize-1)
		binary.Read(r, binary.BigEndian, &encoding)
		binary.Read(r, binary.BigEndian, &information)

		frame.Text = string(reencode(information, encoding))

		return frame, nil
	}

	if header.id[0] == 'W' && header.id != "WXXX" {
		frame := URLLinkFrame{FrameHeader: header}
		url := make([]byte, headerSize)
		binary.Read(r, binary.BigEndian, url)
		frame.URL = string(iso88591ToUTF8(url))

		return frame, nil
	}

	switch header.id {
	case "TXXX":
		return readTXXXFrame(r, header, headerSize), nil
	case "WXXX":
		return readWXXXFrame(r, header, headerSize), nil
	case "UFID":
		return readUFIDFrame(r, header, headerSize), nil
	case "COMM":
		return readCOMMFrame(r, header, headerSize), nil
	default:
		r.Read(make([]byte, headerSize))

		return UnsupportedFrame{header}, nil
	}
}

func New(file *os.File) (*File, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return &File{
		f:        file,
		fileSize: stat.Size(),
		Frames:   make(FramesMap),
	}, nil
}

func Open(name string) (*File, error) {
	f, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	file, err := New(f)
	return file, err
}

func (f *File) Close() error {
	return f.f.Close()
}

// Parse parses the file's tags.
func (f *File) Parse() error {
	header, n, err := readHeader(f.f)
	f.tagReader = io.NewSectionReader(f.f, int64(n), int64(header.Size))
	f.audioReader = io.NewSectionReader(f.f, int64(n)+int64(header.Size), f.fileSize-int64(header.Size))
	if err != nil {
		if _, ok := err.(NotATagHeader); ok {
			return nil
		}
		return err
	}

	f.Header = header
	for {
		frame, err := readFrame(f.tagReader)
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}
		f.Frames[frame.ID()] = append(f.Frames[frame.ID()], frame)
	}

	if f.Header.Version < 0x0400 {
		f.upgrade()
	}
	f.hasTags = true
	return nil
}

// upgrade upgrades tags from an older version to IDv2.4. It should
// only be called for files that use an older version.
func (f *File) upgrade() {
	// Upgrade TYER/TDAT/TIME to TDRC if at least
	// one of TYER, TDAT or TIME are set.
	if f.HasFrame("TYER") || f.HasFrame("TDAT") || f.HasFrame("TIME") {
		Logging.Println("Replacing TYER, TDAT and TIME with TDRC...")

		year := f.GetTextFrameNumber("TYER")
		date := f.GetTextFrame("TDAT")
		t := f.GetTextFrame("TIME")

		if len(date) != 4 {
			date = "0101"
		}

		if len(t) != 4 {
			t = "0000"
		}

		day, _ := strconv.Atoi(date[0:2])
		month, _ := strconv.Atoi(date[2:])
		hour, _ := strconv.Atoi(date[0:2])
		minute, _ := strconv.Atoi(date[2:])

		f.SetRecordingTime(time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC))
		f.RemoveFrames("TYER")
		f.RemoveFrames("TDAT")
		f.RemoveFrames("TIME")
	}

	// Upgrade Original Release Year to Original Release Time
	if !f.HasFrame("TDOR") {
		if f.HasFrame("TORY") {
			Logging.Println("Replacing TORY with TDOR")

			year := f.GetTextFrameNumber("TORY")
			f.SetOriginalReleaseTime(time.Date(year, 0, 0, 0, 0, 0, 0, time.UTC))
		} else if f.HasFrame("XDOR") {
			Logging.Println("Replacing XDOR with TDOR")
			panic("not implemented") // FIXME
		}
	}

	for name, _ := range f.Frames {
		switch name {
		case "TLAN", "TCON", "TPE1", "TOPE", "TCOM", "TEXT", "TOLY":
			f.SetTextFrameSlice(name, strings.Split(f.GetTextFrame(name), "/"))
		}
	}
	// TODO slash -> \x00 as value separator
}

// Clear removes all tags from the file.
func (f *File) Clear() {
	f.Frames = make(FramesMap)
}

func (f *File) RemoveFrames(name FrameType) {
	delete(f.Frames, name)
}

func (f *File) Validate() error {
	panic("not implemented")

	if f.HasFrame("TSRC") && len(f.GetTextFrame("TSRC")) != 12 {
		// TODO invalid TSRC frame
	}

	return nil
}

func (f *File) Album() string {
	return f.GetTextFrame("TALB")
}

func (f *File) SetAlbum(album string) {
	f.SetTextFrame("TALB", album)
}

func (f *File) Artists() []string {
	return f.GetTextFrameSlice("TPE1")
}

func (f *File) SetArtists(artists []string) {
	f.SetTextFrameSlice("TPE1", artists)
}

func (f *File) Artist() string {
	artists := f.Artists()
	if len(artists) > 0 {
		return artists[0]
	}

	return ""
}

func (f *File) SetArtist(artist string) {
	f.SetTextFrame("TPE1", artist)
}

func (f *File) Band() string {
	return f.GetTextFrame("TPE2")
}

func (f *File) SetBand(band string) {
	f.SetTextFrame("TPE2", band)
}

func (f *File) Conductor() string {
	return f.GetTextFrame("TPE3")
}

func (f *File) SetConductor(name string) {
	f.SetTextFrame("TPE3", name)
}

func (f *File) OriginalArtists() []string {
	return f.GetTextFrameSlice("TOPE")
}

func (f *File) SetOriginalArtists(names []string) {
	f.SetTextFrameSlice("TOPE", names)
}

func (f *File) OriginalArtist() string {
	artists := f.OriginalArtists()
	if len(artists) > 0 {
		return artists[0]
	}

	return ""
}

func (f *File) SetOriginalArtist(name string) {
	f.SetTextFrame("TOPE", name)
}

func (f *File) BPM() int {
	return f.GetTextFrameNumber("TBPM")
}

func (f *File) SetBPM(bpm int) {
	f.SetTextFrameNumber("TBPM", bpm)
}

func (f *File) Composers() []string {
	return f.GetTextFrameSlice("TCOM")
}

func (f *File) Title() string {
	return f.GetTextFrame("TIT2")
}

func (f *File) SetTitle(title string) {
	f.SetTextFrame("TIT2", title)
}

func (f *File) Length() time.Duration {
	// TODO if TLEN frame doesn't exist determine the length by
	// parsing the underlying audio file
	return time.Duration(f.GetTextFrameNumber("TLEN")) * time.Millisecond
}

func (f *File) SetLength(d time.Duration) {
	f.SetTextFrameNumber("TLEN", int(d.Nanoseconds()/1e6))
}

func (f *File) Languages() []string {
	return f.GetTextFrameSlice("TLAN")
}

func (f *File) Language() string {
	langs := f.Languages()
	if len(langs) == 0 {
		return ""
	}

	return langs[0]
}

func (f *File) SetLanguages(langs []string) {
	f.SetTextFrameSlice("TLAN", langs)
}

func (f *File) SetLanguage(lang string) {
	f.SetTextFrame("TLAN", lang)
}

func (f *File) Publisher() string {
	return f.GetTextFrame("TPUB")
}

func (f *File) SetPublisher(publisher string) {
	f.SetTextFrame("TPUB", publisher)
}

func (f *File) StationName() string {
	return f.GetTextFrame("TRSN")
}

func (f *File) SetStationName(name string) {
	f.SetTextFrame("TRSN", name)
}

func (f *File) StationOwner() string {
	return f.GetTextFrame("TRSO")
}

func (f *File) SetStationOwner(owner string) {
	f.SetTextFrame("TRSO", owner)
}

func (f *File) Owner() string {
	return f.GetTextFrame("TOWN")
}

func (f *File) SetOwner(owner string) {
	f.SetTextFrame("TOWN", owner)
}

func (f *File) RecordingTime() time.Time {
	return f.GetTextFrameTime("TDRC")
}

func (f *File) SetRecordingTime(t time.Time) {
	f.SetTextFrameTime("TDRC", t)
}

func (f *File) OriginalReleaseTime() time.Time {
	return f.GetTextFrameTime("TDOR")
}

func (f *File) SetOriginalReleaseTime(t time.Time) {
	f.SetTextFrameTime("TDOR", t)
}

func (f *File) OriginalFilename() string {
	return f.GetTextFrame("TOFN")
}

func (f *File) SetOriginalFilename(name string) {
	f.SetTextFrame("TOFN", name)
}

func (f *File) PlaylistDelay() time.Duration {
	return time.Duration(f.GetTextFrameNumber("TDLY")) * time.Millisecond
}

func (f *File) SetPlaylistDelay(d time.Duration) {
	f.SetTextFrameNumber("TDLY", int(d.Nanoseconds()/1e6))
}

func (f *File) EncodingTime() time.Time {
	return f.GetTextFrameTime("TDEN")
}

func (f *File) SetEncodingTime(t time.Time) {
	f.SetTextFrameTime("TDEN", t)
}

func (f *File) AlbumSortOrder() string {
	return f.GetTextFrame("TSOA")
}

func (f *File) SetAlbumSortOrder(s string) {
	f.SetTextFrame("TSOA", s)
}

func (f *File) PerformerSortOrder() string {
	return f.GetTextFrame("TSOP")
}

func (f *File) SetPerformerSortOrder(s string) {
	f.SetTextFrame("TSOP", s)
}

func (f *File) TitleSortOrder() string {
	return f.GetTextFrame("TSOT")
}

func (f *File) SetTitleSortOrder(s string) {
	f.SetTextFrame("TSOT", s)
}

func (f *File) ISRC() string {
	return f.GetTextFrame("TSRC")
}

func (f *File) SetISRC(isrc string) {
	f.SetTextFrame("TSRC", isrc)
}

func (f *File) Mood() string {
	return f.GetTextFrame("TMOO")
}

func (f *File) SetMood(mood string) {
	f.SetTextFrame("TMOO", mood)
}

func (f *File) HasFrame(name FrameType) bool {
	_, ok := f.Frames[name]
	return ok
}

// GetTextFrame returns the text frame specified by name. If it is not
// a valid text frame name (i.e. it does not start with a T or is
// named TXXX), GetTextFrame will panic.
func (f *File) GetTextFrame(name FrameType) string {
	var frames []Frame
	var ok bool
	if name[0] != 'T' {
		frames, ok = f.Frames["TXXX"]
	} else {
		frames, ok = f.Frames[name]
	}

	if !ok {
		return ""
	}

	if name[0] != 'T' {
		// Treat name like it's the description of a TXXX
		for _, frame := range frames {
			userFrame := frame.(UserTextInformationFrame)
			if userFrame.Description == string(name) {
				return userFrame.Text
			}
		}

		return ""
	}

	if len(frames) == 0 {
		return ""
	}

	return frames[0].(TextInformationFrame).Text
}

func (f *File) GetTextFrameNumber(name FrameType) int {
	s := f.GetTextFrame(name)
	if s == "" {
		return 0
	}

	i, _ := strconv.Atoi(s)
	return i
}

func (f *File) GetTextFrameSlice(name FrameType) []string {
	s := f.GetTextFrame(name)
	if s == "" {
		return nil
	}

	return strings.Split(s, "\x00")
}

func (f *File) GetTextFrameTime(name FrameType) time.Time {
	s := f.GetTextFrame(name)
	if s == "" {
		return time.Time{}
	}

	t, err := parseTime(s)
	if err != nil {
		// FIXME figure out a way to signal format errors
		panic(err)
	}

	return t
}

func (f *File) SetTextFrame(name FrameType, value string) {
	if name[0] != 'T' || name == "TXXX" {
		panic("not a valid text frame name: " + name)
	}

	frames, ok := f.Frames[name]
	if !ok {
		frames = make([]Frame, 1)
		f.Frames[name] = frames
	}
	frames[0] = TextInformationFrame{
		FrameHeader: FrameHeader{
			id: name,
		},
		Text: value,
	}
	// TODO what about flags and preserving them?
}

func (f *File) SetTextFrameNumber(name FrameType, value int) {
	f.SetTextFrame(name, strconv.Itoa(value))
}

func (f *File) SetTextFrameSlice(name FrameType, value []string) {
	f.SetTextFrame(name, strings.Join(value, "\x00"))
}

func (f *File) SetTextFrameTime(name FrameType, value time.Time) {
	f.SetTextFrame(name, value.Format(TimeFormat))
}

// TODO all the other methods

func (f *File) CustomFrames() []UserTextInformationFrame {
	res := make([]UserTextInformationFrame, len(f.Frames["TXXX"]))
	for i, frame := range f.Frames["TXXX"] {
		res[i] = frame.(UserTextInformationFrame)
	}

	return res
}

// Save saves the tags to the file. If the changed tags fit into the
// existing file, they will be overwritten in place. Otherwise a new
// file will be created and moved over the old file.
func (f *File) Save() error {
	framesSize := f.Frames.size()

	if f.hasTags && f.Header.Size >= framesSize && len(f.Frames) > 0 {
		// TODO consider writing headers/frames into buffer first, to
		// not break existing file in case of error

		Logging.Println("Writing in-place")

		// The file already has tags and there's enough room to write
		// ours.

		header := generateHeader(f.Header.Size)

		_, err := f.f.Seek(0, 0)
		if err != nil {
			return err
		}

		_, err = f.f.Write(header)
		if err != nil {
			return err
		}

		_, err = f.Frames.WriteTo(f.f)
		if err != nil {
			return err
		}

		f.Header.Version = 0x0400
		// Blank out remainder of previous tags
		_, err = f.f.Write(make([]byte, f.Header.Size-framesSize))
		return err
	} else {
		Logging.Println("Writing new file")
		// We have to create a new file

		var buf io.ReadWriter

		// Work in memory If the old file was smaller than 10MiB, use
		// a temporary file otherwise.
		if f.fileSize < 10*1024*1024 {
			Logging.Println("Working in memory")
			buf = new(bytes.Buffer)
		} else {
			Logging.Println("Using a temporary file")
			newFile, err := ioutil.TempFile("", "id3")
			if err != nil {
				return err
			}
			defer os.Remove(newFile.Name())
			buf = newFile
		}

		_, err := f.WriteTo(buf)
		if err != nil {
			return err
		}

		// We successfully generated a new file, so replace the old
		// one with it.
		err = truncate(f.f)
		if err != nil {
			return err
		}

		if newFile, ok := buf.(*os.File); ok {
			_, err = newFile.Seek(0, 0)
			if err != nil {
				return err
			}
		}

		_, err = io.Copy(f.f, buf)
		if err != nil {
			return err
		}

		f.hasTags = true
		f.Header.Size = framesSize + Padding
		f.Header.Version = 0x0400
		return nil
	}
}

func truncate(f *os.File) error {
	err := f.Truncate(0)
	if err != nil {
		return err
	}
	_, err = f.Seek(0, 0)
	return err
}

func generateHeader(size int) []byte {
	buf := new(bytes.Buffer)

	size = synchsafeInt(size)

	writeMany(buf,
		id3byte,
		versionByte,
		nul, // TODO flags
		intToBytes(size),
	)

	return buf.Bytes()
}

func (fm FramesMap) size() int {
	size := 0
	for _, frames := range fm {
		for _, frame := range frames {
			size += frame.size()
		}
	}

	return size
}

func (fm FramesMap) WriteTo(w io.Writer) (n int64, err error) {
	for _, frames := range fm {
		for _, frame := range frames {
			nw, err := frame.WriteTo(w)
			n += nw
			if err != nil {
				return n, err
			}
		}
	}

	return
}

func (f *File) WriteTo(w io.Writer) (int64, error) {
	var n int64

	if len(f.Frames) > 0 {
		header := generateHeader(f.Frames.size() + Padding)
		n1, err := w.Write(header)
		n += int64(n1)
		if err != nil {
			return n, err
		}

		n2, err := f.Frames.WriteTo(w)
		n += int64(n2)
		if err != nil {
			return n, err
		}

		n1, err = w.Write(make([]byte, Padding))
		n += int64(n1)
		if err != nil {
			return n, err
		}

		_, err = f.audioReader.Seek(0, 0)
		if err != nil {
			return n, err
		}
	}

	// Copy audio data
	n2, err := io.Copy(w, f.audioReader)
	n += int64(n2)
	return n, err
}

func writeMany(w io.Writer, data ...[]byte) (int64, error) {
	n := 0
	for _, data := range data {
		m, err := w.Write(data)
		n += m
		if err != nil {
			return int64(n), err
		}
	}

	return int64(n), nil
}

func desynchsafeInt(b [4]byte) int {
	return int(b[0]<<23) | int(b[1]<<15) | int(b[2])<<7 | int(b[3])
}

func synchsafeInt(i int) int {
	return (i & 0x7f) |
		((i & 0x3f80) << 1) |
		((i & 0x1fc000) << 2) |
		((i & 0xfe0000) << 3)
}

func intToBytes(i int) []byte {
	return []byte{
		byte(i & 0xff000000 >> 24),
		byte(i & 0xff0000 >> 16),
		byte(i & 0xff00 >> 8),
		byte(i & 0xff),
	}
}

func splitNullN(data []byte, encoding Encoding, n int) [][]byte {
	delim := encoding.terminator()
	return bytes.SplitN(data, delim, n)
}

func reencode(b []byte, encoding Encoding) []byte {
	// FIXME: strip trailing null byte
	var ret []byte
	switch encoding {
	case utf16bom, utf16be:
		return utf16ToUTF8(b)
	case utf8:
		ret = make([]byte, len(b))
		copy(ret, b)
		return ret
	case iso88591:
		return iso88591ToUTF8(b)
	}
	panic("unsupported")
}

func utf16ToUTF8(input []byte) []byte {
	// ID3v2 allows UTF-16 in two ways: With a BOM or as Big Endian.
	// So if we have no Little Endian BOM, it has to be Big Endian
	// either way.
	bigEndian := true
	if input[0] == 0xFF && input[1] == 0xFE {
		bigEndian = false
		input = input[2:]
	} else if input[0] == 0xFE && input[1] == 0xFF {
		input = input[2:]
	}

	uint16s := make([]uint16, len(input)/2)

	i := 0
	for j := 0; j < len(input); j += 2 {
		if bigEndian {
			uint16s[i] = uint16(input[j])<<8 | uint16(input[j+1])
		} else {
			uint16s[i] = uint16(input[j]) | uint16(input[j+1])<<8
		}

		i++
	}

	return []byte(string(utf16p.Decode(uint16s)))
}

func iso88591ToUTF8(input []byte) []byte {
	// - ISO-8859-1 bytes match Unicode code points
	// - All runes <128 correspond to ASCII, same as in UTF-8
	// - All runes >128 in ISO-8859-1 encode as 2 bytes in UTF-8
	res := make([]byte, len(input)*2)

	var j int
	for _, b := range input {
		if b <= 128 {
			res[j] = b
			j += 1
		} else {
			if b >= 192 {
				res[j] = 195
				res[j+1] = b - 64
			} else {
				res[j] = 194
				res[j+1] = b
			}
			j += 2
		}
	}

	return res[:j]
}

func utf8ToISO88591(input []byte) []byte {
	res := make([]byte, len(input))
	i := 0

	for j := 0; j < len(input); j++ {
		if input[j] <= 128 {
			res[i] = input[j]
		} else {
			if input[j] == 195 {
				res[i] = input[j+1] + 64
			} else {
				res[i] = input[j+1]
			}
			j++
		}
		i++
	}

	return res[:i]
}

func parseTime(input string) (res time.Time, err error) {
	for _, format := range timeFormats {
		res, err = time.Parse(format, input)
		if err == nil {
			break
		}
	}

	return
}

// TRCK
// The 'Track number/Position in set' frame is a numeric string containing the order number of the audio-file on its original recording. This may be extended with a "/" character and a numeric string containing the total numer of tracks/elements on the original recording. E.g. "4/9".

// TODO update TDTG when writing tags
