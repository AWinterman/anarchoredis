package message

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/awinterman/anarchoredis/protocol/kind"
	"gotest.tools/v3/assert"
)

var EOL = kind.EOL

// TestDecode_SimpleMessage tests the Decode function for simple message types.
func TestDecode_SimpleMessage(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectations func(*testing.T, Message, error)
	}{
		{
			name:  "Decode Simple SimpleString",
			input: fmt.Sprintf("%ctest string%s", kind.SimpleString, EOL),
			expectations: func(t *testing.T, msg Message, err error) {
				assert.Equal(t, msg.SimpleString, "test string")
				assert.Equal(t, msg.Kind, kind.SimpleString)
				assert.Equal(t, msg.TotalMessageSize, int64(len(fmt.Sprintf("%ctest string%s", kind.SimpleString, EOL))))
				assert.NilError(t, err)
			},
		},
		{
			name:  "Decode Integer",
			input: string([]byte{byte(kind.Int)}) + "12345" + EOL,
			expectations: func(t *testing.T, msg Message, err error) {
				assert.Equal(t, msg.Int, int64(12345))
				assert.NilError(t, err)
			},
		},
		{
			name:  "Decode Boolean True",
			input: string([]byte{byte(kind.Bool)}) + "true" + EOL,
			expectations: func(t *testing.T, msg Message, err error) {
				assert.Equal(t, msg.Bool, true)
				assert.NilError(t, err)
			},
		},
		{
			name:  "Invalid Simple Kind - Boolean",
			input: string([]byte{byte(kind.Bool)}) + "notabool" + EOL,
			expectations: func(t *testing.T, msg Message, err error) {
				assert.ErrorIs(t, err, strconv.ErrSyntax)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoder := Encoder{}
			reader := strings.NewReader(test.input)

			msg, err := encoder.Decode(reader)
			test.expectations(t, msg, err)
		})
	}
}

// TestDecode_AggregateMessages tests the Decode function for aggregate message types.
func TestDecode_AggregateMessages(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedRunLength int64
		expectedEncoding  string
		expectErr         bool
		expectedOutput    string
	}{
		{
			name:              "Decode Verbatim SimpleString",
			input:             string([]byte{byte(kind.VerbatimString)}) + "11" + EOL + "txt" + EOL + "hello world",
			expectedRunLength: 11,
			expectedEncoding:  "txt",
			expectErr:         false,
			expectedOutput:    "hello world",
		},
		{
			name:              "Decode Bulk SimpleString",
			input:             string([]byte{byte(kind.BulkString)}) + "5" + EOL + "hello",
			expectedRunLength: 5,
			expectErr:         false,
			expectedOutput:    "hello",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoder := Encoder{}
			reader := bytes.NewBufferString(test.input)

			msg, err := encoder.Decode(reader)
			if test.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if msg.RunLength != test.expectedRunLength {
				t.Errorf("expected run length %d, got %d", test.expectedRunLength, msg.RunLength)
			}

			if test.expectedEncoding != "" && msg.Encoding != [3]byte{test.expectedEncoding[0], test.expectedEncoding[1], test.expectedEncoding[2]} {
				t.Errorf("expected encoding %q, got %q", test.expectedEncoding, msg.Encoding)
			}

			output := make([]byte, msg.RunLength)
			_, err = io.ReadFull(msg.Reader, output)
			assert.NilError(t, err)
			assert.Equal(t, string(output), test.expectedOutput)
		})
	}
}

// TestDecode_InvalidInput tests the Decode function with invalid input.
func TestDecode_InvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "Invalid Kind",
			input:     string([]byte{0xFF}) + "invalid data" + EOL,
			expectErr: true,
		},
		{
			name:      "Empty Input",
			input:     "",
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoder := Encoder{}
			reader := strings.NewReader(test.input)

			m, err := encoder.Decode(reader)
			t.Log(m)
			if test.expectErr && err == nil {
				t.Fatalf("expected error but got none")
			}

			if !test.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestDecode_BigNumber tests decoding a BigNumber message.
func TestDecode_BigNumber(t *testing.T) {
	encoder := Encoder{}
	bigNum := big.NewInt(1234567890)
	input := string([]byte{byte(kind.BigNumber)}) + bigNum.String() + EOL
	reader := strings.NewReader(input)

	msg, err := encoder.Decode(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.Kind != kind.BigNumber {
		t.Errorf("expected kind BigNumber, got %v", msg.Kind)
	}

	if msg.BigNumber.Cmp(bigNum) != 0 {
		t.Errorf("expected big number %v, got %v", bigNum, msg.BigNumber)
	}
}
