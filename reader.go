package validator

import (
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func detectCompression(name string) string {
	switch filepath.Ext(name) {
	case ".gzip", ".gz":
		return "gzip"
	case ".bzip2", ".bz2":
		return "bzip2"
	}

	return ""
}

// Reader encapsulates a stdin stream.
type Reader struct {
	Name        string
	Compression string

	reader io.Reader
	file   *os.File
}

// Read implements the io.Reader interface.
func (r *Reader) Read(buf []byte) (int, error) {
	return r.reader.Read(buf)
}

// Close implements the io.Closer interface.
func (r *Reader) Close() {
	if r.file != nil {
		r.file.Close()
	}
}

// Open a reader by name with optional compression. If no name is specified, STDIN
// is used.
func Open(name, compr string) (*Reader, error) {
	r := new(Reader)

	if compr == "" {
		compr = detectCompression(name)
	}

	// Validate Compressionession method before working with files.
	switch compr {
	case "bzip2", "gzip", "":
	default:
		return nil, fmt.Errorf("unknown compression type %s", compr)
	}

	if name == "" {
		r.reader = os.Stdin
	} else {
		file, err := os.Open(name)

		if err != nil {
			return nil, err
		}

		r.file = file
		r.reader = file
	}

	// Apply the Compressionession decoder.
	switch compr {
	case "gzip":
		reader, err := gzip.NewReader(r.reader)

		if err != nil {
			r.Close()
			return nil, err
		}

		r.reader = reader
	case "bzip2":
		r.reader = bzip2.NewReader(r.reader)
	}

	r.Compression = compr

	return r, nil
}
