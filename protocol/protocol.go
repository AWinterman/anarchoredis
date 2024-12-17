package protocol

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"strings"
	"sync"
)

type Indicator byte

const (
	End = "\r\n"

	Seperator      Indicator = ' '
	String         Indicator = '+' // simple
	Error          Indicator = '-'
	Int            Indicator = ':'
	BulkString     Indicator = '$'
	Array          Indicator = '*'
	Null           Indicator = '_'
	Bool           Indicator = '#'
	Double         Indicator = ','
	BigNumber      Indicator = '('
	BulkError      Indicator = '!'
	VerbatimString Indicator = '='
	Map            Indicator = '%'
	Attribute      Indicator = '`'
	Sets           Indicator = '~'
	Push           Indicator = '>'
)

func (i Indicator) String() string {
	return Humanize(byte(i))
}

// Humanize returns a human readable string for the indicator
func Humanize(indicator byte) string {
	switch Indicator(indicator) {
	case String:
		return "String"
	case Error:
		return "Error"
	case Int:
		return "Int"
	case BulkString:
		return "BulkString"
	case Array:
		return "Array"
	case Null:
		return "Null"
	case Bool:
		return "Bool"
	case Double:
		return "Double"
	case BigNumber:
		return "BigNumber"
	case VerbatimString:
		return "VerbatimString"
	case Map:
		return "Map"
	case Attribute:
		return "Attribute"
	case Sets:
		return "Sets"
	case Push:
		return "Push"
	default:
		return "Unknown"
	}
}

// Message is a composite type that represents a message in the protocol
// the Indicator says which fields should be respected.
//
// where sensible, metadata or original data will be included in the
// struct, for example, the encoding of a VerbatimString
type Message struct {
	Indicator      Indicator
	Str            string
	Error          error
	Int            int
	Array          []*Message    // Arrays also have an Int value that is the number of elements in the array
	Map            [][2]*Message // key value pairs
	Bool           bool
	Double         float64
	BigNumber      *big.Int
	VerbatimString struct {
		Encoding string // 3 byte for the encoding
		Data     string // the data
	}
	OriginalSize int64
}

func (m *Message) String() string {
	return fmt.Sprintf("%s%s", string(byte(m.Indicator)), m.string())
}

func (m *Message) string() string {
	switch m.Indicator {
	case String:
		return m.Str
	case Error:
		return m.Error.Error()
	case Int:
		return fmt.Sprintf("%d", m.Int)
	case Array:
		s := make([]string, len(m.Array))
		for i, msg := range m.Array {
			s[i] = msg.String()
		}
		return fmt.Sprintf("%s", strings.Join(s, " "))
	case BulkString:
		return m.Str
	case Map:
		s := make([]string, len(m.Map))
		for i, msg := range m.Map {
			s[i] = fmt.Sprintf("%s => %s", msg[0], msg[1])
		}
		return fmt.Sprintf("{%s}", strings.Join(s, " "))
	case Bool:
		return fmt.Sprintf("%v", m.Bool)
	case Double:
		return fmt.Sprintf("%v", m.Double)
	case BigNumber:
		return fmt.Sprintf("%v", m.BigNumber)
	case VerbatimString:
		return m.Str
	default:
		return fmt.Sprintf("Unknown %s", m.Indicator)
	}
}

var (
	r = newReader()
	w = newWriter()
)

func Read(conn *bufio.ReadWriter) (*Message, error) {
	return r.Read(conn)
}

func Write(conn *bufio.ReadWriter, m *Message) (int, error) {
	return w.Write(conn, m)
}

type Conn struct {
	sync.Mutex
	rw *bufio.ReadWriter
}

func (conn *Conn) Read() (*Message, error) {
	conn.Lock()
	defer conn.Unlock()
	return r.Read(conn.rw)
}

func (conn *Conn) Write(m *Message) (int, error) {
	conn.Lock()
	defer conn.Unlock()
	return w.Write(conn.rw, m)
}

func (conn *Conn) Flush() error {
	conn.Lock()
	defer conn.Unlock()
	return conn.rw.Flush()
}

func NewConnection(conn io.ReadWriter) *Conn {
	return &Conn{rw: bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))}

}
