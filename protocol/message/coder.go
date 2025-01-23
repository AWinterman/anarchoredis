package message

import (
	"fmt"
	"io"
	"iter"
	"math/big"
	"strconv"

	"github.com/awinterman/anarchoredis/protocol/kind"
)

func simpleUnmarshal(r io.Reader) (str string, size int64, err error) {
	var (
		b   = make([]byte, 1)
		buf []byte
	)

	for {
		n, err := r.Read(b)
		if err != nil {
			return "", int64(len(buf)), err
		}

		buf = append(buf, b[:n]...)

		if len(buf) > len(kind.EOL) {
			l := len(buf) - len(kind.EOL)
			s := string(buf[l:])
			if s == kind.EOL {
				return string(buf[:l]), int64(len(buf)), err
			}
		}
	}
}

type Encoder struct {
	ChunkSize  int64
	BufferSize int
}

func (e *Encoder) Iterate(r io.Reader) iter.Seq2[Message, error] {
	return func(yield func(message Message, err error) bool) {
		for {
			kindSlice := make([]byte, 1)
			_, err := r.Read(kindSlice)
			if err != nil {
				_ = yield(Message{}, err)
				return
			}

			k := kind.Kind(kindSlice[0])
			var m Message
			switch k.Category() {
			case kind.CategorySimple:
				str, size, err := simpleUnmarshal(r)
				if err != nil {
					_ = yield(Message{}, err)
					return
				}
				switch k {
				case kind.SimpleString:
					m = SimpleString(str)
				case kind.Error:
					m = Error(str)
				case kind.Int:
					i, err := strconv.ParseInt(str, 10, 64)
					if err != nil {
						_ = yield(Message{}, err)
						return
					}
					m = Int(i)
				case kind.Null:
					m = Null()
				case kind.Bool:
					b, err := strconv.ParseBool(str)
					if err != nil {
						_ = yield(Message{}, err)
						return
					}
					m = Bool(b)
				case kind.Double:
					f, err := strconv.ParseFloat(str, 64)
					if err != nil {
						_ = yield(Message{}, err)
						return
					}
					m = Double(f)
				case kind.BigNumber:
					i := &big.Int{}
					i, ok := i.SetString(str, 0)
					if !ok {
						_ = yield(Message{}, err)
						return
					}
					m = BigNumber(i)
				}
				m.TotalMessageSize = 1 + size
			case kind.CategoryAggregate:
				// first unmarshal the first line, which has the run length
				str, size, err := simpleUnmarshal(r)
				if err != nil {
					_ = yield(Message{}, err)
					return
				}
				runlength, err := strconv.ParseInt(str, 10, 64)
				if err != nil {
					_ = yield(Message{}, err)
					return
				}

				m.TotalMessageSize = 1 + size + runlength

				var encoding [3]byte
				if k == kind.VerbatimString {
					enc, size, err := simpleUnmarshal(r)
					if err != nil {
						_ = yield(Message{}, err)
						return
					}
					if len(enc) != 3 {
						err = fmt.Errorf("invalid string size: %d", size)
						_ = yield(Message{}, err)
						return
					}
					copy(encoding[:], enc)
					m.TotalMessageSize += 3
				}

				switch k {
				case kind.VerbatimString, kind.BulkString, kind.BulkError:
					m.Reader = io.LimitReader(r, runlength)
				case kind.Array, kind.Set, kind.Push:
					for i := 0; i < int(runlength); i++ {
						m.Seq = e.Iterate(r)
					}
				case kind.Map, kind.Attribute:
					for i := 0; i < int(runlength); i++ {
						kvs := e.Iterate(r)
						m.Assoc = chunk(kvs)
					}
				}

				m.RunLength = runlength
				m.Encoding = encoding
			}
		}
	}
}

func chunk(seq iter.Seq2[Message, error]) iter.Seq2[[2]Message, error] {
	return func(yield func([2]Message, error) bool) {
		var kv [2]Message = [2]Message{}
		var i = 0
		for msg, err := range seq {
			if err != nil {
				yield(kv, err)
			}
			kv[i%2] = msg
			i++
			if i%2 == 0 {
				yield(kv, nil)
			}
		}
	}
}

func (e *Encoder) Decode(r io.Reader) (Message, error) {
	iterate := e.Iterate(r)
	for msg, err := range iterate {
		return msg, err
	}
	return Message{}, fmt.Errorf("reader closed before message could be read")
}

// Encode the message into the Writer
func (e *Encoder) Encode(m Message, w io.Writer) (total int, err error) {
	var n int
	n, err = w.Write([]byte{byte(m.Kind)})
	total += n
	if err != nil {
		return
	}

	switch m.Kind.Category() {
	case kind.CategorySimple:
		var body []byte
		body, err = simpleToByte(m)
		if err != nil {
			return
		}
		n, err = w.Write(body)
		total += n
		if err != nil {
			return
		}
	case kind.CategoryAggregate:
		n, err = w.Write([]byte(strconv.FormatInt(m.RunLength, 10)))
		total += n
		if err != nil {
			return
		}
		n, err = w.Write([]byte(kind.EOL))
		total += n
		if err != nil {
			return
		}

		if m.Kind == kind.VerbatimString {
			n, err = w.Write(m.Encoding[:])
			total += n
			if err != nil {
				return
			}
			n, err = w.Write([]byte(kind.EOL))
			total += n
			if err != nil {
				return
			}
		}

		switch m.Kind {
		case kind.BulkString, kind.BulkError, kind.VerbatimString:
			n, err = e.writeBulk(m, w)
			total += n
			if err != nil {
				return total, err
			}
		case kind.Array, kind.Push, kind.Set:
			for msg, err := range m.Seq {
				if err != nil {
					return total, err
				}
				n, err = e.Encode(msg, w)
				total += n
				if err != nil {
					return total, err
				}
			}
		case kind.Map, kind.Attribute:
			for kv, err := range m.Assoc {
				n, err = e.Encode(kv[0], w)
				total += n
				if err != nil {
					return total, err
				}
				n, err = e.Encode(kv[1], w)
				total += n
				if err != nil {
					return total, err
				}
			}
		}
	}

	return total, err
}

// writeBulk writes a bulk message to the writer
func (e *Encoder) writeBulk(m Message, w io.Writer) (int, error) {
	left := m.RunLength
	var total int
	var n int
	var err error
	for left > 0 {
		b := emptyBuffer(e.ChunkSize, left)
		n, err = m.Reader.Read(b)
		if err != nil {
			return total, err
		}
		n, err = w.Write(b[:n])
		total += n
		if err != nil {
			return total, err
		}
		left = left - int64(n)
	}
	n, err = w.Write([]byte(kind.EOL))
	total += n

	return total, err
}

// simpleToByte converts simple types to bytes.
func simpleToByte(m Message) ([]byte, error) {
	// for simpleToByte types, we just need to string format the relevant type.
	var body []byte
	switch m.Kind {
	case kind.SimpleString:
		body = []byte(m.SimpleString)
	case kind.Int:
		body = []byte(strconv.FormatInt(m.Int, 10))
	case kind.Bool:
		body = []byte(fmt.Sprintf("%t", m.Bool))
	case kind.Error:
		if m.Error != nil {
			return nil, fmt.Errorf("empty Error field for Error type message %s", m)
		}
		body = []byte(m.Error.Error())
	case kind.Double:
		body = []byte(strconv.FormatFloat(m.Double, 'f', -1, 64))
	case kind.BigNumber:
		body = []byte(m.BigNumber.String())
	}

	return append(body, []byte(kind.EOL)...), nil
}

// emptyBuffer creates a byte slice with a length determined by the smaller of chunkSize or totalSize, with defaults applied.
func emptyBuffer(chunkSize, runLength int64) []byte {
	bufferSize := chunkSize
	if chunkSize < 1 {
		bufferSize = 409
	}

	length := runLength
	if runLength > bufferSize {
		length = bufferSize
	}
	return make([]byte, length)
}
