package protocol

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestRead_String(t *testing.T) {
	b := bytes.NewBufferString("+OK\r\n")

	r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
	result, err := newReader().Read(r2)

	assert.NilError(t, err)
	assert.Equal(t, result.Str, "OK") // result should be "OK"
	assert.Equal(t, result.Indicator, String)
}

func TestRead_Error(t *testing.T) {
	b := bytes.NewBufferString("-Error\r\n")
	r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
	result, err := newReader().Read(r2)

	assert.NilError(t, err)
	assert.Equal(t, result.Error.Error(), "Error")
	assert.Equal(t, result.Indicator, Error)
}

func TestRead_Int(t *testing.T) {
	t.Run("an int", func(t *testing.T) {
		b := bytes.NewBufferString(":1024\r\n")

		r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
		result, err := newReader().Read(r2)

		assert.NilError(t, err)
		assert.Equal(t, result.Int, 1024) // result should be "OK"
		assert.Equal(t, string(result.Indicator), string(Int))
	})

	t.Run("not an int", func(t *testing.T) {
		b := bytes.NewBufferString(":Hi\r\n")

		r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
		_, err := newReader().Read(r2)

		assert.ErrorIs(t, err, strconv.ErrSyntax)
	})
}

func TestRead_BulkString(t *testing.T) {
	t.Run("simple case", func(t *testing.T) {
		bulkStringTest(t,
			"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz0123456789\r\n")
	})
	t.Run("with intermediary delim", func(t *testing.T) {
		bulkStringTest(t, "abcdefghijklmnopqrstuvwxyzabcdef\r\nghijklmnopqrstuvwxyz0123456789\r\n")
	})

	t.Run("bulk string reads off the end of the connection", func(t *testing.T) {
		old := slog.SetLogLoggerLevel(slog.LevelDebug)
		defer slog.SetLogLoggerLevel(old)
		data := "abcdefg"
		server, client := net.Pipe()
		go func() {
			defer server.Close()
			// Do some stuff
			server.Write([]byte(fmt.Sprintf("$%d\r\n%s", len(data), data[0:2])))
			time.Sleep(100 * time.Millisecond)
			server.Write([]byte(data[2:]))
			server.Write([]byte("\r\n"))
		}()
		defer client.Close()

		r2 := bufio.NewReadWriter(bufio.NewReader(client), nil)
		result, err := newReader().Read(r2)

		assert.NilError(t, err)
		assert.Equal(t, result.Str, data)
	})

	t.Run("bulk string reads off the end of the connection", func(t *testing.T) {
		old := slog.SetLogLoggerLevel(slog.LevelDebug)
		defer slog.SetLogLoggerLevel(old)
		data := "abcdefg"
		server, client := net.Pipe()
		go func() {
			defer server.Close()
			// Do some stuff
			server.Write([]byte(fmt.Sprintf("$%d", len(data))))
			time.Sleep(10 * time.Millisecond)
			server.Write([]byte("\r\n"))
			time.Sleep(10 * time.Millisecond)
			server.Write([]byte(data))
			time.Sleep(10 * time.Millisecond)
			server.Write([]byte("\r\n"))
		}()
		defer client.Close()

		r2 := bufio.NewReadWriter(bufio.NewReader(client), nil)
		result, err := newReader().Read(r2)

		assert.NilError(t, err)
		assert.Equal(t, result.Str, data)
	})

}

func bulkStringTest(t *testing.T, data string) {
	b := bytes.NewBufferString(fmt.Sprintf("$%d\r\n%s\r\n", len(data), data))

	r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
	result, err := newReader().Read(r2)

	assert.NilError(t, err)
	assert.Equal(t, result.Str, data)
}

func TestRead_Array(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected []*Message
	}{
		"empty array": {
			input:    "*0\r\n",
			expected: nil,
		},
		"bulk strings": {
			input: "*2\r\n$5\r\nhello\r\n$5\r\nworld\r\n",
			expected: []*Message{
				{Str: "hello", Indicator: BulkString, OriginalSize: 7},
				{Str: "world", Indicator: BulkString, OriginalSize: 7},
			},
		},
		"ints": {
			input: "*3\r\n:1\r\n:2\r\n:3\r\n",
			expected: []*Message{
				{Int: 1, Indicator: Int, OriginalSize: 3},
				{Int: 2, Indicator: Int, OriginalSize: 3},
				{Int: 3, Indicator: Int, OriginalSize: 3},
			},
		},
		"mixed types": {
			input: "*5\r\n:1\r\n:2\r\n:3\r\n:4\r\n$5\r\nhello\r\n",
			expected: []*Message{
				{Int: 1, Indicator: Int, OriginalSize: 3},
				{Int: 2, Indicator: Int, OriginalSize: 3},
				{Int: 3, Indicator: Int, OriginalSize: 3},
				{Int: 4, Indicator: Int, OriginalSize: 3},
				{Str: "hello", Indicator: BulkString, OriginalSize: 7},
			},
		},
	}

	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			b := bytes.NewBufferString(testcase.input)

			r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
			result, err := newReader().Read(r2)

			assert.NilError(t, err)
			assert.Equal(t, len(result.Array), len(testcase.expected))
			assert.Equal(t, result.Indicator, Array)
			for i, expected := range testcase.expected {
				assert.DeepEqual(t, result.Array[i], expected)
			}
		})
	}

}

func TestRead_Null(t *testing.T) {
	b := bytes.NewBufferString("_\r\n")

	r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
	result, err := newReader().Read(r2)

	assert.NilError(t, err)
	assert.Equal(t, result.Indicator, Null)
}

func TestRead_Bool(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		b := bytes.NewBufferString("#t\r\n")

		r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
		result, err := newReader().Read(r2)

		assert.NilError(t, err)
		assert.Equal(t, result.Bool, true)
	})

	t.Run("false", func(t *testing.T) {
		b := bytes.NewBufferString("#f\r\n")

		r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
		result, err := newReader().Read(r2)

		assert.NilError(t, err)
		assert.Equal(t, result.Indicator, Bool)
		assert.Equal(t, result.Bool, false)
	})
}

func TestRead_Double(t *testing.T) {
	b := bytes.NewBufferString(",1.23\r\n")

	r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
	result, err := newReader().Read(r2)

	assert.NilError(t, err)
	assert.Equal(t, result.Indicator, Double)
	assert.Equal(t, result.Double, 1.23)
	assert.Equal(t, result.Str, "1.23")
}

func TestRead_Verbatim(t *testing.T) {
	b := bytes.NewBufferString("=15\r\ntxt:Some string\r\n")

	r2 := bufio.NewReadWriter(bufio.NewReader(b), nil)
	result, err := newReader().Read(r2)

	assert.NilError(t, err)
	assert.Equal(t, result.Indicator, VerbatimString)
	assert.Equal(t, result.VerbatimString.Encoding, "txt")
	assert.Equal(t, result.VerbatimString.Data, "Some string")
}
