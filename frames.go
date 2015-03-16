package id3

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

var PictureTypes = []string{
	"Other",
	"32x32 pixels 'file icon' (PNG only)",
	"Other file icon",
	"Cover (front)",
	"Cover (back)",
	"Leaflet page",
	"Media (e.g. label side of CD)",
	"Lead artist/lead performer/soloist",
	"Artist/performer",
	"Conductor",
	"Band/Orchestra",
	"Composer",
	"Lyricist/text writer",
	"Recording Location",
	"During recording",
	"During performance",
	"Movie/video screen capture",
	"A bright coloured fish",
	"Illustration",
	"Band/artist logotype",
	"Publisher/Studio logotype",
}

type FrameHeader struct {
	id    FrameType
	flags FrameFlags
}

func (h FrameHeader) Header() FrameHeader { return h }

type Frame interface {
	ID() FrameType
	Header() FrameHeader
	Value() string
	Encode() []byte
	size() int // TODO export?
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

type PrivateFrame struct {
	FrameHeader
	Owner []byte
	Data  []byte
}

type PictureFrame struct {
	FrameHeader
	MIMEType    string
	PictureType PictureType
	Description string
	Data        []byte
}

type MusicCDIdentifierFrame struct {
	FrameHeader
	TOC []byte
}

type UnsynchronisedLyricsFrame struct {
	FrameHeader
	Language    string
	Description string
	Lyrics      string
}

type UnsupportedFrame struct {
	FrameHeader
	Data []byte
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

func (f TextInformationFrame) Encode() []byte {
	switch f.FrameHeader.ID() {
	case "TRDA", "TSIZ":
		Logging.Println("Not writing header", f.FrameHeader.ID())
		return nil
	default:
		b2 := utf8byte
		b3 := []byte(f.Text)
		var b4 []byte
		b4 = append(b4, b2...)
		b4 = append(b4, b3...)
		return b4
	}
}

func (f TextInformationFrame) Value() string {
	return f.Text
}

func (f UserTextInformationFrame) size() int {
	return frameLength + len(f.Description) + len(f.Text) + 2
}

func (f UserTextInformationFrame) Encode() []byte {
	b2 := utf8byte
	b3 := []byte(f.Description)
	b4 := nul
	b5 := []byte(f.Text)
	var b6 []byte
	b6 = append(b6, b2...)
	b6 = append(b6, b3...)
	b6 = append(b6, b4...)
	b6 = append(b6, b5...)
	return b6
}

func (f UserTextInformationFrame) Value() string {
	return f.Text
}

func (f UniqueFileIdentifierFrame) size() int {
	iso := utf8.toISO88591([]byte(f.Owner))
	return frameLength + len(f.Identifier) + len(iso) + 1
}

func (f UniqueFileIdentifierFrame) Encode() []byte {
	iso := utf8.toISO88591([]byte(f.Owner))
	b2 := iso
	b3 := nul
	b4 := f.Identifier
	var b5 []byte
	b5 = append(b5, b2...)
	b5 = append(b5, b3...)
	b5 = append(b5, b4...)
	return b5
}

func (f UniqueFileIdentifierFrame) Value() string {
	return string(f.Identifier)
}

func (f URLLinkFrame) size() int {
	return frameLength + len(utf8.toISO88591([]byte(f.URL)))
}

func (f URLLinkFrame) Encode() []byte {
	iso := utf8.toISO88591([]byte(f.URL))
	b2 := iso
	var b3 []byte
	b3 = append(b3, b2...)
	return b3
}

func (f URLLinkFrame) Value() string {
	return f.URL
}

func (f UserDefinedURLLinkFrame) size() int {
	iso := utf8.toISO88591([]byte(f.URL))
	return frameLength + len(f.Description) + len(iso) + 2
}

func (f UserDefinedURLLinkFrame) Encode() []byte {
	iso := utf8.toISO88591([]byte(f.URL))
	b2 := utf8byte
	b3 := []byte(f.Description)
	b4 := nul
	b5 := iso
	var b6 []byte
	b6 = append(b6, b2...)
	b6 = append(b6, b3...)
	b6 = append(b6, b4...)
	b6 = append(b6, b5...)
	return b6
}

func (f UserDefinedURLLinkFrame) Value() string {
	return f.URL
}

func (f CommentFrame) size() int {
	return frameLength + len(f.Description) + len(f.Text) + 5
}

func (f CommentFrame) Encode() []byte {
	b2 := utf8byte
	b3 := []byte(f.Language)
	b4 := []byte(f.Description)
	b5 := nul
	b6 := []byte(f.Text)
	var b7 []byte
	b7 = append(b7, b2...)
	b7 = append(b7, b3...)
	b7 = append(b7, b4...)
	b7 = append(b7, b5...)
	b7 = append(b7, b6...)
	return b7
}

func (f CommentFrame) Value() string {
	return f.Text
}

func (f PrivateFrame) Value() string {
	return string(f.Data)
}

func (f PrivateFrame) size() int {
	return frameLength + len(f.Owner) + len(f.Data) + len(nul)
}

func (f PrivateFrame) Encode() []byte {
	b2 := f.Owner
	b3 := nul
	b4 := f.Data
	var b5 []byte
	b5 = append(b5, b2...)
	b5 = append(b5, b3...)
	b5 = append(b5, b4...)
	return b5
}

func (f PictureFrame) Value() string {
	return string(f.Data)
}

func (f PictureFrame) size() int {
	return frameLength +
		1 +
		len(utf8.toISO88591([]byte(f.MIMEType))) +
		len(nul) +
		1 +
		len(f.Description) +
		len(nul) +
		len(f.Data)
}

func (f PictureFrame) Encode() []byte {
	b2 := utf8byte
	b3 := utf8.toISO88591([]byte(f.MIMEType))
	b4 := nul
	b5 := []byte{byte(f.PictureType)}
	b6 := []byte(f.Description)
	b7 := nul
	b8 := f.Data
	var b9 []byte
	b9 = append(b9, b2...)
	b9 = append(b9, b3...)
	b9 = append(b9, b4...)
	b9 = append(b9, b5...)
	b9 = append(b9, b6...)
	b9 = append(b9, b7...)
	b9 = append(b9, b8...)
	return b9
}

func (f MusicCDIdentifierFrame) Value() string {
	return string(f.TOC)
}

func (f MusicCDIdentifierFrame) size() int {
	return frameLength + len(f.TOC)
}

func (f MusicCDIdentifierFrame) Encode() []byte {
	b2 := f.TOC
	var b3 []byte
	b3 = append(b3, b2...)
	return b3
}

func (f UnsynchronisedLyricsFrame) Value() string {
	return f.Lyrics
}

func (f UnsynchronisedLyricsFrame) size() int {
	return frameLength + 5 + len(f.Description) + len(f.Lyrics)
}

func (f UnsynchronisedLyricsFrame) Encode() []byte {
	b2 := utf8byte
	b3 := []byte(f.Language)
	b4 := []byte(f.Description)
	b5 := nul
	b6 := []byte(f.Lyrics)
	var b7 []byte
	b7 = append(b7, b2...)
	b7 = append(b7, b3...)
	b7 = append(b7, b4...)
	b7 = append(b7, b5...)
	b7 = append(b7, b6...)
	return b7
}

func (f UnsupportedFrame) size() int {
	return frameLength + len(f.Data)
}

func (f UnsupportedFrame) Encode() []byte {
	// TODO check header if unsupported frame should be dropped or copied verbatim
	b2 := f.Data
	var b3 []byte
	b3 = append(b3, b2...)
	return b3
}

func (UnsupportedFrame) Value() string {
	// TODO return raw data
	return ""
}
