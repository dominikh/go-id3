package id3

import (
	"bytes"
	"testing"
	"time"
)

var (
	UTF8TestString  = []byte("Ein etwas kürzerer Text mit wenigen Umlauten: äöüß äöüß")
	UTF16TestString = []byte{254, 255, 0, 69, 0, 105, 0, 110, 0, 32,
		0, 101, 0, 116, 0, 119, 0, 97, 0, 115, 0, 32, 0, 107, 0, 252, 0,
		114, 0, 122, 0, 101, 0, 114, 0, 101, 0, 114, 0, 32, 0, 84, 0, 101,
		0, 120, 0, 116, 0, 32, 0, 109, 0, 105, 0, 116, 0, 32, 0, 119, 0,
		101, 0, 110, 0, 105, 0, 103, 0, 101, 0, 110, 0, 32, 0, 85, 0, 109,
		0, 108, 0, 97, 0, 117, 0, 116, 0, 101, 0, 110, 0, 58, 0, 32, 0,
		228, 0, 246, 0, 252, 0, 223, 0, 32, 0, 228, 0, 246, 0, 252, 0,
		223}
	ISOTestString = []byte("Ein etwas k\xFCrzerer Text mit wenigen Umlauten: \xE4\xF6\xFC\xDF \xE4\xF6\xFC\xDF")
)

func TestUTF8ToISO88591(t *testing.T) {
	in := []byte("Ein etwas kürzerer Text mit wenigen Umlauten: äöüß äöüß")
	out := []byte("Ein etwas k\xFCrzerer Text mit wenigen Umlauten: \xE4\xF6\xFC\xDF \xE4\xF6\xFC\xDF")

	res := utf8ToISO88591(in)

	if !bytes.Equal(res, out) {
		t.Fail()
	}
}

func TestISO88591ToUTF8(t *testing.T) {
	in := []byte("Ein etwas k\xFCrzerer Text mit wenigen Umlauten: \xE4\xF6\xFC\xDF \xE4\xF6\xFC\xDF")
	out := []byte("Ein etwas kürzerer Text mit wenigen Umlauten: äöüß äöüß")

	res := iso88591ToUTF8(in)

	if !bytes.Equal(res, out) {
		t.Fail()
	}
}

func TestUTF16ToUTF8(t *testing.T) {
	in := []byte{254, 255, 0, 74, 0,
		117, 0, 115, 0, 116, 0, 32, 0, 97, 0, 32, 0, 116, 0, 101, 0, 115,
		0, 116, 0, 58, 0, 32, 0, 228, 0, 252, 0, 246, 0, 32, 101, 229,
		103, 44, 138, 158}
	out := []byte("Just a test: äüö 日本語")

	res := utf16bom.toUTF8(in)

	if !bytes.Equal(res, out) {
		t.Errorf("Expected: %s - Got: %s", out, res)
	}
}

func TestUTF16BEToUTF8(t *testing.T) {
	in := []byte{0, 74, 0,
		117, 0, 115, 0, 116, 0, 32, 0, 97, 0, 32, 0, 116, 0, 101, 0, 115,
		0, 116, 0, 58, 0, 32, 0, 228, 0, 252, 0, 246, 0, 32, 101, 229,
		103, 44, 138, 158}
	out := []byte("Just a test: äüö 日本語")

	res := utf16be.toUTF8(in)

	if !bytes.Equal(res, out) {
		t.Errorf("Expected: %s - Got: %s", out, res)
	}
}

func TestUTF16LEToUTF8(t *testing.T) {
	in := []byte{255, 254, 74, 0, 117, 0, 115, 0, 116, 0, 32, 0, 97,
		0, 32, 0, 116, 0, 101, 0, 115, 0, 116, 0, 58, 0, 32, 0, 228, 0,
		252, 0, 246, 0, 32, 0, 229, 101, 44, 103, 158, 138}

	out := []byte("Just a test: äüö 日本語")

	res := utf16bom.toUTF8(in)

	if !bytes.Equal(res, out) {
		t.Errorf("Expected: %s - Got: %s", out, res)
	}
}

func TestTimeParsing(t *testing.T) {
	tests := []struct {
		in  string
		out time.Time
	}{
		{"2009-11-10T23:01:02", time.Date(2009, 11, 10, 23, 01, 02, 0, time.UTC)},
		{"2009-11-10T23:01", time.Date(2009, 11, 10, 23, 01, 0, 0, time.UTC)},
		{"2009-11-10T23", time.Date(2009, 11, 10, 23, 0, 0, 0, time.UTC)},
		{"2009-11-10", time.Date(2009, 11, 10, 0, 0, 0, 0, time.UTC)},
		{"2009-11", time.Date(2009, 11, 1, 0, 0, 0, 0, time.UTC)},
		{"2009", time.Date(2009, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, test := range tests {
		res, err := parseTime(test.in)
		if err != nil {
			t.Fatalf("Couldn't parse time '%s': %s", test.in, err)
		}

		if res != test.out {
			t.Fatalf("Time '%s' parsed to '%s' instead of '%s'", test.in, res, test.out)
		}
	}
}

func TestUserFrameNameParsing(t *testing.T) {
	tests := []struct {
		in      FrameType
		outName string
		outBool bool
	}{
		{"TLEN", "", false},
		{"TXXX:", "", false},
		{"TXXX:User frame", "User frame", true},
	}

	for _, test := range tests {
		out, ok := frameNameToUserFrame(test.in)
		if out != test.outName || ok != test.outBool {
			t.Fatalf("Didn't parse user frame name correctly. Expected: %q/%t, got %q/%t",
				test.outName, test.outBool, out, ok)
		}
	}
}

func BenchmarkISO88591ToUTF8(b *testing.B) {
	b.SetBytes(int64(len(ISOTestString)))
	for i := 0; i < b.N; i++ {
		_ = iso88591ToUTF8(ISOTestString)
	}
}

func BenchmarkUTF8ToISO88591(b *testing.B) {
	b.SetBytes(int64(len(UTF8TestString)))
	for i := 0; i < b.N; i++ {
		_ = utf8ToISO88591(UTF8TestString)
	}
}

func BenchmarkUTF16ToUTF8(b *testing.B) {
	b.SetBytes(int64(len(UTF16TestString)))
	for i := 0; i < b.N; i++ {
		_ = utf16ToUTF8(UTF16TestString)
	}
}

func ExampleTag_GetTextFrame_text(t *Tag) {
	t.GetTextFrame("TIT2") // Same as f.Title()
}

func ExampleTag_GetTextFrame_user(t *Tag) {
	t.GetTextFrame("TXXX:MusicBrainz Album Artist Id")
}
