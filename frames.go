package id3

import (
	"bytes"
	"encoding/binary"
	"io"
)

var FrameNames = map[FrameType]string{
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
	"TORY": "Original release year",
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

type FrameHeader struct {
	id    FrameType
	flags FrameFlags
}

type Frame interface {
	ID() FrameType
	io.WriterTo
	size() int
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

func (f FrameHeader) ID() FrameType {
	return f.id
}

func (f FrameHeader) serialize(size int) []byte {
	out := make([]byte, 10)
	copy(out, f.id)

	flagBytes := intToBytes(int(f.flags))
	copy(out[8:10], flagBytes[2:4])

	sizeBytes := intToBytes(synchsafeInt(size))
	copy(out[4:8], sizeBytes)

	return out
}

func (f TextInformationFrame) size() int {
	if f.FrameHeader.ID() == "TRDA" {
		return 0
	}

	return frameLength + len(f.Text) + 1
}

func (f TextInformationFrame) WriteTo(w io.Writer) (int64, error) {
	if f.FrameHeader.ID() == "TRDA" {
		Logging.Println("Skipping TRDA header")
		return 0, nil
	}

	return writeMany(w,
		f.FrameHeader.serialize(f.size()-frameLength),
		utf8byte,
		[]byte(f.Text),
	)
}

func (f UserTextInformationFrame) size() int {
	return frameLength + len(f.Description) + len(f.Text) + 2
}

func (f UserTextInformationFrame) WriteTo(w io.Writer) (int64, error) {
	return writeMany(w,
		f.FrameHeader.serialize(f.size()-frameLength),
		utf8byte,
		[]byte(f.Description),
		nul,
		[]byte(f.Text),
	)
}

func (f UniqueFileIdentifierFrame) size() int {
	iso := utf8ToISO88591([]byte(f.Owner))
	return frameLength + len(f.Identifier) + len(iso) + 1
}

func (f UniqueFileIdentifierFrame) WriteTo(w io.Writer) (int64, error) {
	iso := utf8ToISO88591([]byte(f.Owner))
	return writeMany(w,
		f.FrameHeader.serialize(f.size()-frameLength),
		iso,
		nul,
		f.Identifier,
	)
}

func (f URLLinkFrame) size() int {
	return frameLength + len(utf8ToISO88591([]byte(f.URL)))
}

func (f URLLinkFrame) WriteTo(w io.Writer) (int64, error) {
	iso := utf8ToISO88591([]byte(f.URL))
	return writeMany(w,
		f.FrameHeader.serialize(f.size()-frameLength),
		iso,
	)
}

func (f UserDefinedURLLinkFrame) size() int {
	iso := utf8ToISO88591([]byte(f.URL))
	return frameLength + len(f.Description) + len(iso) + 2
}

func (f UserDefinedURLLinkFrame) WriteTo(w io.Writer) (int64, error) {
	iso := utf8ToISO88591([]byte(f.URL))
	return writeMany(w,
		f.FrameHeader.serialize(f.size()-frameLength),
		utf8byte,
		[]byte(f.Description),
		nul,
		iso,
	)
}

func (f CommentFrame) size() int {
	return frameLength + len(f.Description) + len(f.Text) + 5
}

func (f CommentFrame) WriteTo(w io.Writer) (int64, error) {
	return writeMany(w,
		f.FrameHeader.serialize(f.size()-frameLength),
		utf8byte,
		[]byte(f.Language),
		[]byte(f.Description),
		nul,
		[]byte(f.Text),
	)
}

func (UnsupportedFrame) size() int {
	return 0
}

func (f UnsupportedFrame) WriteTo(w io.Writer) (int64, error) {
	Logging.Println("Cannot serialize unsupported frame:", f)
	// TODO remove println
	// TODO check if unsupported frame should be dropped or copied verbatim
	return 0, nil
}

func readTXXXFrame(r io.Reader, header FrameHeader, headerSize int) Frame {
	var encoding Encoding
	frame := UserTextInformationFrame{FrameHeader: header}
	binary.Read(r, binary.BigEndian, &encoding)
	rest := make([]byte, headerSize-1)
	binary.Read(r, binary.BigEndian, &rest)
	parts := splitNullN(rest, encoding, 2)
	frame.Description = string(reencode(parts[0], encoding))
	frame.Text = string(reencode(parts[1], encoding))

	return frame
}

func readWXXXFrame(r io.Reader, header FrameHeader, headerSize int) Frame {
	var encoding Encoding
	frame := UserDefinedURLLinkFrame{FrameHeader: header}
	binary.Read(r, binary.BigEndian, &encoding)
	rest := make([]byte, headerSize-1)
	binary.Read(r, binary.BigEndian, &rest)
	parts := splitNullN(rest, encoding, 2)
	frame.Description = string(reencode(parts[0], encoding))
	frame.URL = string(iso88591ToUTF8(parts[1]))

	return frame
}

func readUFIDFrame(r io.Reader, header FrameHeader, headerSize int) Frame {
	frame := UniqueFileIdentifierFrame{FrameHeader: header}
	rest := make([]byte, headerSize)
	binary.Read(r, binary.BigEndian, &rest)
	parts := bytes.SplitN(rest, []byte{0}, 2)
	frame.Owner = string(reencode(parts[0], iso88591))
	frame.Identifier = parts[1]

	return frame
}

func readCOMMFrame(r io.Reader, header FrameHeader, headerSize int) Frame {
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

	return frame
}
