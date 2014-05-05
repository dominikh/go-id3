package id3

import (
	"fmt"
	utf16pkg "unicode/utf16"
)

const (
	iso88591 Encoding = iota
	utf16bom
	utf16be
	utf8
)

var (
	utf16nul = []byte{0, 0}
	nul      = []byte{0}
	utf8byte = []byte{byte(utf8)}
)

type Encoding byte

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

func (e Encoding) toUTF8(b []byte) []byte {
	var ret []byte
	switch e {
	case utf16bom, utf16be:
		ret = utf16ToUTF8(b)
	case utf8:
		ret = make([]byte, len(b))
		copy(ret, b)
	case iso88591:
		ret = iso88591ToUTF8(b)
	default:
		panic("unsupported")
	}

	if len(ret) > 0 && ret[len(ret)-1] == 0 {
		return ret[:len(ret)-1]
	}

	return ret
}

func (e Encoding) toISO88591(b []byte) []byte {
	if e != utf8 {
		panic("Conversion to ISO-8859-1 is only implemented for UTF-8")
	}

	return utf8ToISO88591(b)
}

func (e Encoding) terminator() []byte {
	switch e {
	case utf16bom, utf16be:
		return utf16nul
	default:
		return nul
	}
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

	return []byte(string(utf16pkg.Decode(uint16s)))
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

func iso88591ToUTF8(input []byte) []byte {
	// - ISO-8859-1 bytes match Unicode code points
	// - All runes <128 correspond to ASCII, same as in UTF-8
	// - All runes >128 in ISO-8859-1 encode as 2 bytes in UTF-8
	res := make([]byte, len(input)*2)

	var j int
	for _, b := range input {
		if b <= 128 {
			res[j] = b
			j++
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
