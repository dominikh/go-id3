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
	ID() FrameType // TODO consider renaming to Type()
	Header() FrameHeader
	Value() string
	Encode() []byte
	Size() int
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

func (f TextInformationFrame) Size() int {
	if f.FrameHeader.ID() == "TRDA" {
		return 0
	}

	return frameLength + len(f.Text) + 1
}

func (f TextInformationFrame) Encode() []byte {
	switch f.FrameHeader.ID() {
	case "TRDA", "TSIZ":
		return nil
	default:
		return concat(utf8byte, []byte(f.Text))
	}
}

func (f TextInformationFrame) Value() string {
	return f.Text
}

func (f UserTextInformationFrame) Size() int {
	return frameLength + len(f.Description) + len(f.Text) + 2
}

func (f UserTextInformationFrame) Encode() []byte {
	return concat(utf8byte, []byte(f.Description), nul, []byte(f.Text))
}

func (f UserTextInformationFrame) Value() string {
	return f.Text
}

func (f UniqueFileIdentifierFrame) Size() int {
	iso := utf8.toISO88591([]byte(f.Owner))
	return frameLength + len(f.Identifier) + len(iso) + 1
}

func (f UniqueFileIdentifierFrame) Encode() []byte {
	iso := utf8.toISO88591([]byte(f.Owner))
	return concat(iso, nul, f.Identifier)
}

func (f UniqueFileIdentifierFrame) Value() string {
	return string(f.Identifier)
}

func (f URLLinkFrame) Size() int {
	return frameLength + len(utf8.toISO88591([]byte(f.URL)))
}

func (f URLLinkFrame) Encode() []byte {
	iso := utf8.toISO88591([]byte(f.URL))
	return iso
}

func (f URLLinkFrame) Value() string {
	return f.URL
}

func (f UserDefinedURLLinkFrame) Size() int {
	iso := utf8.toISO88591([]byte(f.URL))
	return frameLength + len(f.Description) + len(iso) + 2
}

func (f UserDefinedURLLinkFrame) Encode() []byte {
	iso := utf8.toISO88591([]byte(f.URL))
	return concat(utf8byte, []byte(f.Description), nul, iso)
}

func (f UserDefinedURLLinkFrame) Value() string {
	return f.URL
}

func (f CommentFrame) Size() int {
	return frameLength + len(f.Description) + len(f.Text) + 5
}

func (f CommentFrame) Encode() []byte {
	return concat(utf8byte, []byte(f.Language), []byte(f.Description), nul, []byte(f.Text))
}

func (f CommentFrame) Value() string {
	return f.Text
}

func (f PrivateFrame) Value() string {
	return string(f.Data)
}

func (f PrivateFrame) Size() int {
	return frameLength + len(f.Owner) + len(f.Data) + len(nul)
}

func (f PrivateFrame) Encode() []byte {
	return concat(f.Owner, nul, f.Data)
}

func (f PictureFrame) Value() string {
	return string(f.Data)
}

func (f PictureFrame) Size() int {
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
	return concat(utf8byte, utf8.toISO88591([]byte(f.MIMEType)), nul,
		[]byte{byte(f.PictureType)}, []byte(f.Description), nul, f.Data)
}

func (f MusicCDIdentifierFrame) Value() string {
	return string(f.TOC)
}

func (f MusicCDIdentifierFrame) Size() int {
	return frameLength + len(f.TOC)
}

func (f MusicCDIdentifierFrame) Encode() []byte {
	return f.TOC
}

func (f UnsynchronisedLyricsFrame) Value() string {
	return f.Lyrics
}

func (f UnsynchronisedLyricsFrame) Size() int {
	return frameLength + 5 + len(f.Description) + len(f.Lyrics)
}

func (f UnsynchronisedLyricsFrame) Encode() []byte {
	return concat(utf8byte, []byte(f.Language), []byte(f.Description), nul, []byte(f.Lyrics))
}

func (f UnsupportedFrame) Size() int {
	return frameLength + len(f.Data)
}

func (f UnsupportedFrame) Encode() []byte {
	// TODO check header if unsupported frame should be dropped or copied verbatim
	return f.Data
}

func (f UnsupportedFrame) Value() string {
	return string(f.Data)
}
