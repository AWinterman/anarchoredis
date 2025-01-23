package protocol

import "io"

type BufferedReader interface {
	Reader
	ReadBytes(delim byte) ([]byte, error)
}

type Reader = io.Reader
