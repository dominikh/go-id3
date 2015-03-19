package id3

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// TODO reevaluate TagHeader. Right now it's a snapshot of the past
// that doesn't reflect the present

// TODO make padding configurable per tag?

// Enables logging if set to true.
var Logging LogFlag

var Magic = [3]byte{0x49, 0x44, 0x33}

// TODO make this configurable per file?

type LogFlag bool

func (l LogFlag) Println(args ...interface{}) {
	if l {
		log.Println(args...)
	}
}

const (
	frameLength   = 10
	tagHeaderSize = 10
)

var (
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
type FrameType string
type FramesMap map[FrameType][]Frame
type PictureType byte

type NotAFrameHeader struct {
	Bytes struct {
		ID    [4]byte
		Size  [4]byte
		Flags [2]byte
	}
}

// TODO export this error?
type notATagHeader struct {
	Magic [3]byte
}

type UnsupportedVersion struct {
	Version Version
}

type TagHeader struct {
	Version Version // The ID3v2 version the file currently has on disk
	Flags   HeaderFlags
	size    int // The size of the tag (exluding the size of the header)
}

type Tag struct {
	Header TagHeader
	Frames FramesMap
}

type Comment struct {
	Language    string
	Description string
	Text        string
}

func concat(bs ...[]byte) []byte {
	n := 0
	for _, b := range bs {
		n += len(b)
	}
	out := make([]byte, 0, n)
	for _, b := range bs {
		out = append(out, b...)
	}
	return out
}

// NewTag returns an empty tag.
func NewTag() *Tag {
	return &Tag{Frames: make(FramesMap)}
}

func (f FrameType) String() string {
	v, ok := FrameNames[f]
	if ok {
		return v
	}

	return string(f)
}

func (p PictureType) String() string {
	if int(p) >= len(PictureTypes) {
		return ""
	}

	return PictureTypes[p]
}

// TODO: HeaderFlags.String()
// TODO: FrameFlags.String()

func (err notATagHeader) Error() string {
	return fmt.Sprintf("Not an ID3v2 header: %q", err.Magic)
}

func (err NotAFrameHeader) Error() string {
	return fmt.Sprintf("Not a frame header (ID = %q)", err.Bytes.ID)
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

// upgrade upgrades tags from an older version to IDv2.4. It should
// only be called for files that use an older version.
func (t *Tag) upgrade() {
	// Upgrade TYER/TDAT/TIME to TDRC if at least
	// one of TYER, TDAT or TIME are set.
	if t.HasFrame("TYER") || t.HasFrame("TDAT") || t.HasFrame("TIME") {
		Logging.Println("Replacing TYER, TDAT and TIME with TDRC...")

		year := t.GetTextFrameNumber("TYER")
		date := t.GetTextFrame("TDAT")
		tim := t.GetTextFrame("TIME")

		if len(date) != 4 {
			date = "0101"
		}

		if len(tim) != 4 {
			tim = "0000"
		}

		day, _ := strconv.Atoi(date[0:2])
		month, _ := strconv.Atoi(date[2:])
		hour, _ := strconv.Atoi(date[0:2])
		minute, _ := strconv.Atoi(date[2:])

		t.SetRecordingTime(time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC))
		t.RemoveFrames("TYER")
		t.RemoveFrames("TDAT")
		t.RemoveFrames("TIME")
	}

	// Upgrade Original Release Year to Original Release Time
	if !t.HasFrame("TDOR") {
		if t.HasFrame("XDOR") {
			Logging.Println("Replacing XDOR with TDOR")
			panic("not implemented") // FIXME
		} else if t.HasFrame("TORY") {
			Logging.Println("Replacing TORY with TDOR")

			year := t.GetTextFrameNumber("TORY")
			t.SetOriginalReleaseTime(time.Date(year, 0, 0, 0, 0, 0, 0, time.UTC))
		}
	}

	for name := range t.Frames {
		switch name {
		case "TLAN", "TCON", "TPE1", "TOPE", "TCOM", "TEXT", "TOLY":
			Logging.Println("Replacing / with x00 for", name)
			t.SetTextFrameSlice(name, strings.Split(t.GetTextFrame(name), "/"))
		}
	}
	// TODO EQUA → EQU2
	// TODO IPL → TMCL, TIPL
	// TODO RVAD → RVA2
	// TODO TRDA → TDRL
}

// Clear removes all tags from the file.
func (t *Tag) Clear() {
	t.Frames = make(FramesMap)
}

func (t *Tag) RemoveFrames(name FrameType) {
	delete(t.Frames, name)
}

// Validate checks whether the tags are conforming to the
// specification.
//
// This entails two checks: Whether only frames that are covered by
// the specification are present and whether all values are within
// valid ranges.
//
// It is well possible that reading existing files will result in
// invalid tags.
//
// Calling Save() will not automatically validate the tags and will
// happily write invalid tags.
//
// Assuming that the original file was valid and that only the
// getter/setter methods were used the generated tags should always be
// valid.
func (t *Tag) Validate() error {
	// TODO consider returning a list of errors, one per invalid frame,
	// specifying the reason

	panic("not implemented") // FIXME

	if t.HasFrame("TSRC") && len(t.GetTextFrame("TSRC")) != 12 {
		// TODO invalid TSRC frame
	}

	return nil
}

// Sanitize will remove all frames that aren't valid. Check the
// documentation of (*File).Validate() to see what "valid" means.
func (t *Tag) Sanitize() {
	panic("not implemented") // FIXME
}

func (t *Tag) Album() string {
	return t.GetTextFrame("TALB")
}

func (t *Tag) SetAlbum(album string) {
	t.SetTextFrame("TALB", album)
}

func (t *Tag) Artists() []string {
	return t.GetTextFrameSlice("TPE1")
}

func (t *Tag) SetArtists(artists []string) {
	t.SetTextFrameSlice("TPE1", artists)
}

func (t *Tag) Artist() string {
	artists := t.Artists()
	if len(artists) > 0 {
		return artists[0]
	}

	return ""
}

func (t *Tag) SetArtist(artist string) {
	t.SetTextFrame("TPE1", artist)
}

func (t *Tag) Band() string {
	return t.GetTextFrame("TPE2")
}

func (t *Tag) SetBand(band string) {
	t.SetTextFrame("TPE2", band)
}

func (t *Tag) Conductor() string {
	return t.GetTextFrame("TPE3")
}

func (t *Tag) SetConductor(name string) {
	t.SetTextFrame("TPE3", name)
}

func (t *Tag) OriginalArtists() []string {
	return t.GetTextFrameSlice("TOPE")
}

func (t *Tag) SetOriginalArtists(names []string) {
	t.SetTextFrameSlice("TOPE", names)
}

func (t *Tag) OriginalArtist() string {
	artists := t.OriginalArtists()
	if len(artists) > 0 {
		return artists[0]
	}

	return ""
}

func (t *Tag) SetOriginalArtist(name string) {
	t.SetTextFrame("TOPE", name)
}

func (t *Tag) BPM() int {
	return t.GetTextFrameNumber("TBPM")
}

func (t *Tag) SetBPM(bpm int) {
	t.SetTextFrameNumber("TBPM", bpm)
}

func (t *Tag) Composers() []string {
	return t.GetTextFrameSlice("TCOM")
}

func (t *Tag) SetComposers(composers []string) {
	t.SetTextFrameSlice("TCOM", composers)
}

func (t *Tag) Composer() string {
	composers := t.Composers()
	if len(composers) > 0 {
		return composers[0]
	}

	return ""
}

func (t *Tag) SetComposer(composer string) {
	t.SetTextFrame("TCOM", composer)
}

func (t *Tag) Title() string {
	return t.GetTextFrame("TIT2")
}

func (t *Tag) SetTitle(title string) {
	t.SetTextFrame("TIT2", title)
}

func (t *Tag) Length() time.Duration {
	// TODO if TLEN frame doesn't exist determine the length by
	// parsing the underlying audio file
	return time.Duration(t.GetTextFrameNumber("TLEN")) * time.Millisecond
}

func (t *Tag) SetLength(d time.Duration) {
	t.SetTextFrameNumber("TLEN", int(d.Nanoseconds()/1e6))
}

func (t *Tag) Languages() []string {
	return t.GetTextFrameSlice("TLAN")
}

func (t *Tag) Language() string {
	langs := t.Languages()
	if len(langs) == 0 {
		return ""
	}

	return langs[0]
}

func (t *Tag) SetLanguages(langs []string) {
	t.SetTextFrameSlice("TLAN", langs)
}

func (t *Tag) SetLanguage(lang string) {
	t.SetTextFrame("TLAN", lang)
}

func (t *Tag) Publisher() string {
	return t.GetTextFrame("TPUB")
}

func (t *Tag) SetPublisher(publisher string) {
	t.SetTextFrame("TPUB", publisher)
}

func (t *Tag) StationName() string {
	return t.GetTextFrame("TRSN")
}

func (t *Tag) SetStationName(name string) {
	t.SetTextFrame("TRSN", name)
}

func (t *Tag) StationOwner() string {
	return t.GetTextFrame("TRSO")
}

func (t *Tag) SetStationOwner(owner string) {
	t.SetTextFrame("TRSO", owner)
}

func (t *Tag) Owner() string {
	return t.GetTextFrame("TOWN")
}

func (t *Tag) SetOwner(owner string) {
	t.SetTextFrame("TOWN", owner)
}

func (t *Tag) RecordingTime() time.Time {
	return t.GetTextFrameTime("TDRC")
}

func (t *Tag) SetRecordingTime(rt time.Time) {
	t.SetTextFrameTime("TDRC", rt)
}

func (t *Tag) OriginalReleaseTime() time.Time {
	return t.GetTextFrameTime("TDOR")
}

func (t *Tag) SetOriginalReleaseTime(rt time.Time) {
	t.SetTextFrameTime("TDOR", rt)
}

func (t *Tag) OriginalFilename() string {
	return t.GetTextFrame("TOFN")
}

func (t *Tag) SetOriginalFilename(name string) {
	t.SetTextFrame("TOFN", name)
}

func (t *Tag) PlaylistDelay() time.Duration {
	return time.Duration(t.GetTextFrameNumber("TDLY")) * time.Millisecond
}

func (t *Tag) SetPlaylistDelay(d time.Duration) {
	t.SetTextFrameNumber("TDLY", int(d.Nanoseconds()/1e6))
}

func (t *Tag) EncodingTime() time.Time {
	return t.GetTextFrameTime("TDEN")
}

func (t *Tag) SetEncodingTime(et time.Time) {
	t.SetTextFrameTime("TDEN", et)
}

func (t *Tag) AlbumSortOrder() string {
	return t.GetTextFrame("TSOA")
}

func (t *Tag) SetAlbumSortOrder(s string) {
	t.SetTextFrame("TSOA", s)
}

func (t *Tag) PerformerSortOrder() string {
	return t.GetTextFrame("TSOP")
}

func (t *Tag) SetPerformerSortOrder(s string) {
	t.SetTextFrame("TSOP", s)
}

func (t *Tag) TitleSortOrder() string {
	return t.GetTextFrame("TSOT")
}

func (t *Tag) SetTitleSortOrder(s string) {
	t.SetTextFrame("TSOT", s)
}

func (t *Tag) ISRC() string {
	return t.GetTextFrame("TSRC")
}

func (t *Tag) SetISRC(isrc string) {
	t.SetTextFrame("TSRC", isrc)
}

func (t *Tag) Mood() string {
	return t.GetTextFrame("TMOO")
}

func (t *Tag) SetMood(mood string) {
	t.SetTextFrame("TMOO", mood)
}

func (t *Tag) Comments() []Comment {
	frames := t.Frames["COMM"]
	comments := make([]Comment, len(frames))

	for i, frame := range frames {
		comment := frame.(CommentFrame)
		comments[i] = Comment{
			Language:    comment.Language,
			Description: comment.Description,
			Text:        comment.Text,
		}
	}

	return comments
}

func (t *Tag) SetComments(comments []Comment) {
	frames := make([]Frame, len(comments))
	for i, comment := range comments {
		frames[i] = CommentFrame{
			FrameHeader: FrameHeader{
				id: "COMM",
			},
			Language:    comment.Language,
			Description: comment.Description,
			Text:        comment.Text,
		}
	}
	t.Frames["COMM"] = frames
}

func (t *Tag) HasFrame(name FrameType) bool {
	_, ok := t.Frames[name]
	return ok
}

// GetTextFrame returns the text frame specified by name.
//
// To access user text frames, specify the name like "TXXX:The
// description".
func (t *Tag) GetTextFrame(name FrameType) string {
	userFrameName, ok := frameNameToUserFrame(name)
	if ok {
		return t.getUserTextFrame(userFrameName)
	}

	// Get normal text frame
	frames := t.Frames[name]
	if len(frames) == 0 {
		return ""
	}

	return frames[0].Value()
}

func (t *Tag) getUserTextFrame(name string) string {
	frames, ok := t.Frames["TXXX"]
	if !ok {
		return ""
	}

	for _, frame := range frames {
		userFrame := frame.(UserTextInformationFrame)
		if userFrame.Description == name {
			return userFrame.Text
		}
	}

	return ""
}

func (t *Tag) GetTextFrameNumber(name FrameType) int {
	s := t.GetTextFrame(name)
	if s == "" {
		return 0
	}

	i, _ := strconv.Atoi(s)
	return i
}

func (t *Tag) GetTextFrameSlice(name FrameType) []string {
	s := t.GetTextFrame(name)
	if s == "" {
		return nil
	}

	return strings.Split(s, "\x00")
}

func (t *Tag) GetTextFrameTime(name FrameType) time.Time {
	s := t.GetTextFrame(name)
	if s == "" {
		return time.Time{}
	}

	ft, err := parseTime(s)
	if err != nil {
		// FIXME figure out a way to signal format errors
		panic(err)
	}

	return ft
}

func (t *Tag) SetTextFrame(name FrameType, value string) {
	userFrameName, ok := frameNameToUserFrame(name)
	if ok {
		t.setUserTextFrame(userFrameName, value)
		return
	}

	frames, ok := t.Frames[name]
	if !ok {
		frames = make([]Frame, 1)
		t.Frames[name] = frames
	}
	frames[0] = TextInformationFrame{
		FrameHeader: FrameHeader{
			id: name,
		},
		Text: value,
	}
	// TODO what about flags and preserving them?
}

func (t *Tag) setUserTextFrame(name string, value string) {
	// Set/create a user text frame
	frame := UserTextInformationFrame{
		FrameHeader: FrameHeader{id: "TXXX"},
		Description: name,
		Text:        value,
	}

	frames, ok := t.Frames["TXXX"]
	if !ok {
		frames = make([]Frame, 0)
		t.Frames["TXXX"] = frames
	}

	var i int
	for i = range frames {
		if frames[i].(UserTextInformationFrame).Description == name {
			ok = true
			break
		}
	}

	if ok {
		frames[i] = frame
	} else {
		t.Frames["TXXX"] = append(t.Frames["TXXX"], frame)
	}

}

func (t *Tag) SetTextFrameNumber(name FrameType, value int) {
	t.SetTextFrame(name, strconv.Itoa(value))
}

func (t *Tag) SetTextFrameSlice(name FrameType, value []string) {
	t.SetTextFrame(name, strings.Join(value, "\x00"))
}

func (t *Tag) SetTextFrameTime(name FrameType, value time.Time) {
	t.SetTextFrame(name, value.Format(TimeFormat))
}

// TODO all the other methods

// UserTextFrames returns all TXXX frames.
func (t *Tag) UserTextFrames() []UserTextInformationFrame {
	res := make([]UserTextInformationFrame, len(t.Frames["TXXX"]))
	for i, frame := range t.Frames["TXXX"] {
		res[i] = frame.(UserTextInformationFrame)
	}

	return res
}

func (fm FramesMap) size() int {
	size := 0
	for _, frames := range fm {
		for _, frame := range frames {
			size += frame.Size()
		}
	}

	return size
}

func writeMany(w io.Writer, data ...[]byte) error {
	for _, data := range data {
		_, err := w.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
}

func desynchsafeInt(b [4]byte) int {
	return int(b[0])<<21 | int(b[1])<<14 | int(b[2])<<7 | int(b[3])
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
	if encoding == utf8 || encoding == iso88591 {
		return bytes.SplitN(data, nul, n)
	}

	var (
		matches [][]byte
		prev    int
	)

	for i := 0; i < len(data); i += 2 {
		// TODO if there's no data[i+1] then this is malformed data
		// and we should return an error
		if data[i] == 0 && data[i+1] == 0 {
			matches = append(matches, data[prev:i])

			if len(matches) == n-1 {
				break
			}
		}
	}

	if prev < len(data)-1 {
		matches = append(matches, data[prev:])
	}

	return matches
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

func truncate(f *os.File) error {
	err := f.Truncate(0)
	if err != nil {
		return err
	}
	_, err = f.Seek(0, 0)
	return err
}

func frameNameToUserFrame(name FrameType) (frameName string, ok bool) {
	if len(name) < 6 {
		return "", false
	}

	if name[0:4] != "TXXX" {
		return "", false
	}

	return string(name[5:]), true
}

// TRCK
// The 'Track number/Position in set' frame is a numeric string containing the order number of the audio-file on its original recording. This may be extended with a "/" character and a numeric string containing the total numer of tracks/elements on the original recording. E.g. "4/9".
