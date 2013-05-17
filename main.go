package main

import (
	"bytes"
	"code.google.com/p/go-charset/charset"
	_ "code.google.com/p/go-charset/data"
	"encoding/binary"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

var _ = spew.Dump

const (
	iso88591 = 0
	utf16bom = 1
	utf16be  = 2
	utf8     = 3
)

var FrameNames = map[string]string{
	"AENC": "Audio encryption",
	"APIC": "Attached picture",
	"ASPI": "Audio seek point index",
	"COMM": "Comments",
	"COMR": "Commercial frame",

	"ENCR": "Encryption method registration",
	"EQU2": "Equalisation (2)",
	"ETCO": "Event timing codes",

	"GEOB": "General encapsulated object",
	"GRID": "Group identification registration",

	"LINK": "Linked information",

	"MCDI": "Music CD identifier",
	"MLLT": "MPEG location lookup table",

	"OWNE": "Ownership frame",

	"PRIV": "Private frame",
	"PCNT": "Play counter",
	"POPM": "Popularimeter",
	"POSS": "Position synchronisation frame",

	"RBUF": "Recommended buffer size",
	"RVA2": "Relative volume adjustment (2)",
	"RVRB": "Reverb",

	"SEEK": "Seek frame",
	"SIGN": "Signature frame",
	"SYLT": "Synchronised lyric/text",
	"SYTC": "Synchronised tempo codes",

	"TALB": "Album/Movie/Show title",
	"TBPM": "BPM (beats per minute)",
	"TCOM": "Composer",
	"TCON": "Content type",
	"TCOP": "Copyright message",
	"TDEN": "Encoding time",
	"TDLY": "Playlist delay",
	"TDOR": "Original release time",
	"TDRC": "Recording time",
	"TDRL": "Release time",
	"TDTG": "Tagging time",
	"TENC": "Encoded by",
	"TEXT": "Lyricist/Text writer",
	"TFLT": "File type",
	"TIPL": "Involved people list",
	"TIT1": "Content group description",
	"TIT2": "Title/songname/content description",
	"TIT3": "Subtitle/Description refinement",
	"TKEY": "Initial key",
	"TLAN": "Language(s)",
	"TLEN": "Length",
	"TMCL": "Musician credits list",
	"TMED": "Media type",
	"TMOO": "Mood",
	"TOAL": "Original album/movie/show title",
	"TOFN": "Original filename",
	"TOLY": "Original lyricist(s)/text writer(s)",
	"TOPE": "Original artist(s)/performer(s)",
	"TOWN": "File owner/licensee",
	"TPE1": "Lead performer(s)/Soloist(s)",
	"TPE2": "Band/orchestra/accompaniment",
	"TPE3": "Conductor/performer refinement",
	"TPE4": "Interpreted, remixed, or otherwise modified by",
	"TPOS": "Part of a set",
	"TPRO": "Produced notice",
	"TPUB": "Publisher",
	"TRCK": "Track number/Position in set",
	"TRSN": "Internet radio station name",
	"TRSO": "Internet radio station owner",
	"TSOA": "Album sort order",
	"TSOP": "Performer sort order",
	"TSOT": "Title sort order",
	"TSO2": "Album Artist sort order", // iTunes extension
	"TSOC": "Composer sort oder",      // iTunes extension
	"TSRC": "ISRC (international standard recording code)",
	"TSSE": "Software/Hardware and settings used for encoding",
	"TSST": "Set subtitle",
	"TYER": "Year",
	"TXXX": "User defined text information frame",

	"UFID": "Unique file identifier",
	"USER": "Terms of use",
	"USLT": "Unsynchronised lyric/text transcription",

	"WCOM": "Commercial information",
	"WCOP": "Copyright/Legal information",
	"WOAF": "Official audio file webpage",
	"WOAR": "Official artist/performer webpage",
	"WOAS": "Official audio source webpage",
	"WORS": "Official Internet radio station homepage",
	"WPAY": "Payment",
	"WPUB": "Publishers official webpage",
	"WXXX": "User defined URL link frame",
}

// TODO: ID3v2 extended header
// TODO: unsynchronisation

type HeaderFlags byte
type FrameFlags int16
type Version int16
type Encoding byte
type FrameType string

func (f FrameType) String() string {
	v, ok := FrameNames[string(f)]
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
		return fmt.Sprintf("%d", byte(e))
	}
}

func (e Encoding) terminator() []byte {
	switch e {
	case utf16bom, utf16be:
		return []byte{0, 0}
	default:
		return []byte{0}
	}
}

// TODO: HeaderFlags.String()
// TODO: FrameFlags.String()

type NotATagHeader struct {
	Magic [3]byte
}

func (NotATagHeader) Error() string {
	return "Not an ID3v2 header"
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

type TagHeader struct {
	Version Version
	Flags   HeaderFlags
	Size    int
}

type FrameHeader struct {
	id    FrameType
	flags FrameFlags
}

func (f FrameHeader) ID() FrameType {
	return f.id
}

type Frame interface {
	ID() FrameType
}

type TextInformationFrame struct {
	FrameHeader
	Text string
}

type UserTextInformationFrame struct {
	FrameHeader
	Description string
	Text        string
}

type UniqueFileIdentifierFrame struct {
	FrameHeader
	Owner      string
	Identifier []byte
}

type URLLinkFrame struct {
	FrameHeader
	URL string
}

type UserDefinedURLLinkFrame struct {
	FrameHeader
	Description string
	URL         string
}

type CommentFrame struct {
	FrameHeader
	Language    string
	Description string
	Text        string
}

type UnsupportedFrame struct {
	FrameHeader
}

func desynchsafeInt(b [4]byte) int {
	return int(b[0]<<23) | int(b[1]<<15) | int(b[2])<<7 | int(b[3])
}

// readHeader reads an ID3v2 header. It expects the reader to be
// seeked to the beginning of the header.
func readHeader(r io.Reader) (TagHeader, error) {
	var (
		bytes struct {
			Magic   [3]byte
			Version [2]byte
			Flags   byte
			Size    [4]byte
		}
		header TagHeader
	)

	binary.Read(r, binary.BigEndian, &bytes.Magic)
	if bytes.Magic != [3]byte{0x49, 0x44, 0x33} {
		return TagHeader{}, NotATagHeader{bytes.Magic}
	}
	binary.Read(r, binary.BigEndian, &bytes.Version)
	binary.Read(r, binary.BigEndian, &bytes.Flags)
	binary.Read(r, binary.BigEndian, &bytes.Size)

	header.Version = Version(int16(bytes.Version[0])<<8 | int16(bytes.Version[1]))
	header.Flags = HeaderFlags(bytes.Flags)
	header.Size = desynchsafeInt(bytes.Size)

	return header, nil
}

func splitNullN(data []byte, encoding Encoding, n int) [][]byte {
	delim := encoding.terminator()
	return bytes.SplitN(data, delim, n)
}

func reencode(b []byte, encoding Encoding) []byte {
	// TODO: truncate after null byte
	switch encoding {
	case utf16bom:
		translator, err := charset.TranslatorFrom("UTF16")
		if err != nil {
			// FIXME return an error
			panic(err)
		}

		_, cdata, err := translator.Translate(b, true)
		if err != nil {
			// FIXME return an error
			panic(err)
		}
		ret := make([]byte, len(cdata))
		copy(ret, cdata)

		return ret
	case utf16be:
		translator, err := charset.TranslatorFrom("UTF16BE")
		if err != nil {
			// FIXME return an error
			panic(err)
		}

		_, cdata, err := translator.Translate(b, true)
		if err != nil {
			// FIXME return an error
			panic(err)
		}
		ret := make([]byte, len(cdata))
		copy(ret, cdata)

		return ret
	case utf8:
		return b
	case iso88591:
		return iso88591ToUTF8(b)
	}
	panic("unsupported")
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

	// TODO what if there's no padding and we're reading audio data?

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
		var encoding Encoding
		frame := UserTextInformationFrame{FrameHeader: header}
		binary.Read(r, binary.BigEndian, &encoding)
		rest := make([]byte, headerSize-1)
		binary.Read(r, binary.BigEndian, &rest)
		parts := splitNullN(rest, encoding, 2)
		frame.Description = string(reencode(parts[0], encoding))
		frame.Text = string(reencode(parts[1], encoding))

		return frame, nil
	case "WXXX":
		var encoding Encoding
		frame := UserDefinedURLLinkFrame{FrameHeader: header}
		binary.Read(r, binary.BigEndian, &encoding)
		rest := make([]byte, headerSize-1)
		binary.Read(r, binary.BigEndian, &rest)
		parts := splitNullN(rest, encoding, 2)
		frame.Description = string(reencode(parts[0], encoding))
		frame.URL = string(iso88591ToUTF8(parts[1]))

		return frame, nil
	case "UFID":
		frame := UniqueFileIdentifierFrame{FrameHeader: header}
		rest := make([]byte, headerSize)
		binary.Read(r, binary.BigEndian, &rest)
		parts := bytes.SplitN(rest, []byte{0}, 2)
		frame.Owner = string(reencode(parts[0], iso88591))
		frame.Identifier = parts[1]

		return frame, nil
	case "COMM":
		frame := CommentFrame{FrameHeader: header}
		var (
			encoding Encoding
			language [3]byte
			rest     []byte
		)
		rest = make([]byte, headerSize-4)

		binary.Read(r, binary.BigEndian, &encoding)
		binary.Read(r, binary.BigEndian, &language)
		binary.Read(r, binary.BigEndian, &rest)

		parts := splitNullN(rest, encoding, 2)

		frame.Language = string(language[:])
		frame.Description = string(reencode(parts[0], encoding))
		frame.Text = string(reencode(parts[1], encoding))

		return frame, nil
	default:
		fmt.Println(header.ID)
		r.Read(make([]byte, headerSize))

		return UnsupportedFrame{header}, nil
	}

	panic("unsupported frame")
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

type File struct {
	f      *os.File
	Header TagHeader
	Frames map[FrameType][]Frame
}

func New(file *os.File) *File {
	return &File{
		f:      file,
		Frames: make(map[FrameType][]Frame),
	}
}

func (f *File) Parse() error {
	header, err := readHeader(f.f)
	if err != nil {
		return err
	}

	f.Header = header

	for {
		frame, err := readFrame(f.f)
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}
		f.Frames[frame.ID()] = append(f.Frames[frame.ID()], frame)
	}

	return nil
}

func (f *File) Validate() error {
	panic("not implemented")
}

func (f *File) Album() string {
	return f.GetTextFrame("TALB")
}

func (f *File) BPM() int {
	return f.GetTextFrameAsNumber("TBPM")
}

func (f *File) Composers() []string {
	return f.GetTextFrameAsSlice("TCOM")
}

func (f *File) Title() string {
	return f.GetTextFrame("TIT2")
}

func (f *File) Length() time.Duration {
	return time.Duration(f.GetTextFrameAsNumber("TLEN")) * time.Millisecond
}

func (f *File) Publisher() string {
	return f.GetTextFrame("TPUB")
}

func (f *File) Owner() string {
	return f.GetTextFrame("TOWN")
}

func (f *File) Year() int {
	return f.GetTextFrameAsNumber("TYER")
}

func (f *File) GetTextFrame(name FrameType) string {
	frame, ok := f.Frames[name]
	if !ok {
		return ""
	}

	return frame[0].(TextInformationFrame).Text
}

func (f *File) GetTextFrameAsNumber(name FrameType) int {
	s := f.GetTextFrame(name)
	if s == "" {
		return 0
	}

	i, _ := strconv.Atoi(s)
	return i
}

func (f *File) GetTextFrameAsSlice(name FrameType) []string {
	s := f.GetTextFrame(name)
	if s == "" {
		return nil
	}

	return strings.Split(s, "/")
}

// TODO all the other methods

func main() {
	file := "test.mp3"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}

	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}

	tags := New(f)
	err = tags.Parse()
	if err != nil {
		panic(err)
	}

	fmt.Println("Album:", tags.Album())
	fmt.Println("Title:", tags.Title())
	fmt.Println("Length:", tags.Length())
	fmt.Println("Date:", tags.GetTextFrame("TDAT"))
}
