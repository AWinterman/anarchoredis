package protocol

import (
	"bufio"
	"fmt"
)

type writer map[Indicator]func(*bufio.ReadWriter, *Message) (int, error)

func (w *writer) Write(conn *bufio.ReadWriter, m *Message) (int, error) {
	if f, ok := (*w)[m.Indicator]; !ok {
		return 0, fmt.Errorf("unknown indicator %q", string(m.Indicator))
	} else {
		return f(conn, m)
	}
}

func (w *writer) array(conn *bufio.ReadWriter, m *Message) (int, error) {
	n, err := conn.Write([]byte(fmt.Sprintf("%c%d\r\n", m.Indicator, len(m.Array))))
	if err != nil {
		return n, err
	}
	for _, msg := range m.Array {
		nn, err := w.Write(conn, msg)
		if err != nil {
			return nn, err
		}
		n += nn
	}
	return n, nil
}

func (w *writer) string(conn *bufio.ReadWriter, m *Message) (int, error) {
	return conn.Write([]byte(fmt.Sprintf("%c%s\r\n", m.Indicator, m.Str)))
}

func newWriter() *writer {
	w := make(writer)
	w[Array] = w.array
	w[String] = w.string
	w[Error] = w.string
	w[Double] = w.string
	w[BigNumber] = w.string
	w[Int] = func(conn *bufio.ReadWriter, m *Message) (int, error) {
		return conn.Write([]byte(fmt.Sprintf("%c%d\r\n", m.Indicator, m.Int)))
	}
	w[BulkString] = func(conn *bufio.ReadWriter, m *Message) (int, error) {
		return conn.Write([]byte(fmt.Sprintf("%c%d\r\n%s\r\n", m.Indicator, len(m.Str), m.Str)))
	}
	w[Null] = func(conn *bufio.ReadWriter, m *Message) (int, error) {
		return conn.Write([]byte(fmt.Sprintf("%c\r\n", m.Indicator)))
	}
	w[Bool] = func(conn *bufio.ReadWriter, m *Message) (int, error) {
		return conn.Write([]byte(fmt.Sprintf("%c%s\r\n", m.Indicator, map[bool]string{true: "t", false: "f"}[m.Bool])))
	}
	w[VerbatimString] = func(conn *bufio.ReadWriter, m *Message) (int, error) {
		return conn.Write([]byte(fmt.Sprintf("%c%s:%s\r\n", m.Indicator, m.VerbatimString.Encoding, m.VerbatimString.Data)))
	}
	w[Map] = func(conn *bufio.ReadWriter, m *Message) (int, error) {
		n, err := conn.Write([]byte(fmt.Sprintf("%c%d\r\n", m.Indicator, len(m.Map))))
		if err != nil {
			return n, err
		}
		for _, pair := range m.Map {
			nn, err := w.Write(conn, pair[0])
			if err != nil {
				return n, err
			}
			n += nn
			nn, err = w.Write(conn, pair[1])
			if err != nil {
				return n, err
			}
		}
		return n, nil
	}
	w[Sets] = w.array
	w[Push] = w.array

	return &w
}
