package web

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"regexp"
	"unicode/utf8"

	"github.com/cenoda/gscouter/internal/enka"
)

// Builds travel as base64url text: a selection's builds in its link
// (selection.String), and each showcase character's build on the front
// page, which the browser keeps after the character leaves the showcase
// (handleShowcase).

var uidPattern = regexp.MustCompile(`^[1-9][0-9]{8,9}$`)

// maxPacked bounds the packed text a link or a browser may send.
const maxPacked = 4 << 10

var errBuilds = errors.New("malformed builds")

// packBuilds writes a selection's nickname and builds, compressed: four
// characters take about 300 characters of URL.
func packBuilds(nickname string, builds []enka.Character) string {
	b := binary.AppendUvarint(nil, uint64(len(nickname)))
	b = append(b, nickname...)
	b = append(b, enka.Pack(builds)...)
	var z bytes.Buffer
	w, _ := flate.NewWriter(&z, flate.BestCompression)
	w.Write(b)
	w.Close()
	return base64.RawURLEncoding.EncodeToString(z.Bytes())
}

func unpackBuilds(s string) (nickname string, builds []enka.Character, err error) {
	if len(s) > maxPacked {
		return "", nil, errBuilds
	}
	z, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", nil, errBuilds
	}
	// Packed builds are small; don't inflate more than a little.
	b, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(z)), 64<<10))
	if err != nil {
		return "", nil, errBuilds
	}
	n, size := binary.Uvarint(b)
	if size <= 0 || n > 64 || uint64(len(b)-size) < n {
		return "", nil, errBuilds
	}
	nickname = string(b[size : size+int(n)])
	if !utf8.ValidString(nickname) {
		return "", nil, errBuilds
	}
	builds, err = enka.Unpack(b[size+int(n):])
	return nickname, builds, err
}

// buildToken is one character's build, for the browser to keep.
func buildToken(c enka.Character) string {
	return base64.RawURLEncoding.EncodeToString(enka.Pack([]enka.Character{c}))
}

func parseBuildToken(s string) (enka.Character, error) {
	if len(s) > maxPacked {
		return enka.Character{}, errBuilds
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return enka.Character{}, errBuilds
	}
	chars, err := enka.Unpack(b)
	if err != nil || len(chars) != 1 {
		return enka.Character{}, errBuilds
	}
	return chars[0], nil
}
