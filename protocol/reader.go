package protocol

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"strings"
)

type reader map[Indicator]func(conn *bufio.ReadWriter) (*Message, error)

func newReader() *reader {
	r := make(reader)
	r[String] = r.string
	r[Error] = r.error
	r[Int] = r.int
	r[BulkString] = r.bulkString
	r[Array] = r.array
	r[Null] = r.null
	r[Bool] = r.bool
	r[Double] = r.double
	r[BigNumber] = r.bignum
	r[Map] = r._map
	r[Sets] = r.set
	r[VerbatimString] = r.verbatimstring
	r[BulkError] = r.bulkerror
	r[Push] = r.push
	return &r
}

var traceLevel = slog.Level(-8)

func (r *reader) Read(conn *bufio.ReadWriter) (*Message, error) {
	for {
		t, err := conn.ReadByte()
		slog.Log(context.Background(), traceLevel, "read type", "bytes", string(t), "error", err)

		if t == '\r' || t == '\n' {
			continue
		}
		if err != nil {
			return nil, err
		}
		if f, ok := (*r)[Indicator(t)]; !ok {
			errUnknownType := fmt.Errorf("unknown indicator %q", string(t))
			return nil, errUnknownType
		} else {
			r, err := f(conn)
			if err != nil {
				return nil, err
			}
			return r, nil
		}
	}
}

// string parses a simple string from the connection
func (r *reader) string(conn *bufio.ReadWriter) (*Message, error) {
	m := &Message{Indicator: String}
	line, err := conn.ReadBytes('\n')
	slog.Log(context.Background(), traceLevel, "read line", "bytes", string(line), "error", err)

	if err != nil {
		return nil, err
	}
	if len(line) < 2 {
		return nil, errors.New("missing CRLF")
	}

	line = line[:len(line)-2]

	m.Str = string(line)
	m.OriginalSize += int64(len(line) + 2)
	return m, err
}

// null parses a null message from the connection
func (r *reader) null(conn *bufio.ReadWriter) (*Message, error) {
	s, err := r.string(conn)
	if err != nil {
		return nil, err
	}

	if s.Str != "" {
		return nil, errors.New("null message should be empty")
	}
	return &Message{
		Indicator: Null,
	}, err
}

// error parses an error message from the connection
func (r *reader) error(conn *bufio.ReadWriter) (*Message, error) {
	m, err := r.string(conn)
	if err != nil {
		return nil, err
	}
	m.Error = errors.New(m.Str)
	m.Indicator = Error
	return m, err
}

// int parses an integer from the connection
func (r *reader) int(conn *bufio.ReadWriter) (*Message, error) {
	m, err := r.string(conn)
	if err != nil {
		return nil, err
	}
	m.Int, err = strconv.Atoi(m.Str)
	if err != nil {
		return nil, err
	}
	m.Indicator = Int
	m.Str = ""
	return m, err
}

func (r *reader) bulkString(conn *bufio.ReadWriter) (*Message, error) {
	m := &Message{Indicator: BulkString}
	count, err := r.int(conn)
	if err != nil {
		return m, fmt.Errorf("%w reading int in bulkstring", err)
	}
	if count.Int == 0 {
		return m, nil
	}

	var n = 0
	for n < count.Int {
		var b = make([]byte, count.Int-n)
		s, err := conn.Read(b)
		n += s
		if err != nil {
			return nil, err
		}
		m.Str += string(b[0:s])
		m.OriginalSize += int64(s)
	}

	var eol = make([]byte, len(End))
	var l = 0
	s, err := conn.Read(eol)
	l += s
	m.OriginalSize += int64(s)
	if err != nil {
		return nil, err
	}

	return m, err
}

func (r *reader) bulkerror(conn *bufio.ReadWriter) (*Message, error) {
	m, err := r.bulkString(conn)
	if err != nil {
		return nil, err
	}
	m.Error = errors.New(m.Str)
	m.Indicator = Error
	return m, err
}

func (r *reader) array(conn *bufio.ReadWriter) (*Message, error) {
	m := &Message{Indicator: Array}
	count, err := r.int(conn)
	if err != nil {
		return nil, err
	}

	for i := 0; i < count.Int; i++ {
		msg, err := r.Read(conn)
		if err != nil {
			return nil, fmt.Errorf("%w reading int in array %q", err, m)
		}
		m.Array = append(m.Array, msg)
		m.OriginalSize += msg.OriginalSize
	}

	return m, err
}

func (r *reader) set(conn *bufio.ReadWriter) (*Message, error) {
	m, err := r.array(conn)
	if err != nil {
		return nil, err
	}
	m.Indicator = Sets
	return m, err
}

func (r *reader) push(conn *bufio.ReadWriter) (*Message, error) {
	m, err := r.array(conn)
	if err != nil {
		return nil, err
	}
	m.Indicator = Push
	return m, err
}

func (r *reader) bool(conn *bufio.ReadWriter) (*Message, error) {
	s, err := r.string(conn)
	if err != nil {
		return nil, err
	}

	m := &Message{Indicator: Bool}

	switch s.Str {
	case "t":
		m.Bool = true
	case "f":
		m.Bool = false
	default:
		return nil, fmt.Errorf("unexpected boolean value %s", s.Str)
	}

	return m, nil
}

func (r *reader) double(conn *bufio.ReadWriter) (*Message, error) {
	m, err := r.string(conn)
	if err != nil {
		return nil, err
	}
	m.Indicator = Double
	m.Double, err = strconv.ParseFloat(m.Str, 64)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (r *reader) bignum(conn *bufio.ReadWriter) (*Message, error) {
	m, err := r.string(conn)
	if err != nil {
		return nil, err
	}
	b := big.NewInt(0)
	b.SetString(m.Str, 10)
	m.Indicator = BigNumber
	m.BigNumber = b
	return m, nil
}

func (r *reader) _map(conn *bufio.ReadWriter) (*Message, error) {
	m := &Message{Indicator: Map}
	count, err := r.int(conn)
	if err != nil {
		return nil, err
	}

	for i := 0; i < count.Int; i++ {
		key, err := r.Read(conn)
		if err != nil {
			return nil, err
		}
		m.OriginalSize += key.OriginalSize
		val, err := r.Read(conn)
		if err != nil {
			return nil, err
		}
		m.OriginalSize += val.OriginalSize
		m.Map = append(m.Map, [2]*Message{key, val})
	}

	return m, err
}

// verbatiem strings: =<length>\r\n<encoding>:<data>\r\n
func (r *reader) verbatimstring(conn *bufio.ReadWriter) (*Message, error) {
	m, err := r.bulkString(conn)
	if err != nil {
		return nil, err
	}

	pair := strings.SplitN(m.Str, ":", 2)
	m.VerbatimString.Encoding = pair[0]
	m.VerbatimString.Data = pair[1]
	m.Indicator = VerbatimString

	return m, err
}
