package message

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/awinterman/anarchoredis/protocol/kind"
	"gotest.tools/v3/assert"
)

// TestEncode_SimpleMessage tests the Encode function for a simple type message
func TestEncode_SimpleMessage(t *testing.T) {
	encoder := Encoder{}
	buffer := &bytes.Buffer{}

	message := SimpleString("Hello, world!")

	total, err := encoder.Encode(message, buffer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedOutput := string([]byte{byte(message.Kind)}) + "Hello, world!" + kind.EOL
	if buffer.String() != expectedOutput {
		t.Errorf("expected output %q, got %q", expectedOutput, buffer.String())
	}

	if total != len(expectedOutput) {
		t.Errorf("expected total bytes written %d, got %d", len(expectedOutput), total)
	}
}

// TestEncode_AggregateMessages tests the Encode function for aggregate types like Array, Push, etc.
func TestEncode_AggregateMessages(t *testing.T) {
	bulkdata := "bulkdata"
	tests := []struct {
		name           string
		message        Message
		expectedOutput string
	}{
		{
			name:           "Test bulk string",
			message:        BulkString(strings.NewReader(bulkdata), int64(len(bulkdata))),
			expectedOutput: fmt.Sprintf("$%d%s%s%s", len(bulkdata), kind.EOL, bulkdata, kind.EOL),
		},
		{
			name:           "Test bulk error",
			message:        BulkError(strings.NewReader(bulkdata), int64(len(bulkdata))),
			expectedOutput: fmt.Sprintf("!%d%s%s%s", len(bulkdata), kind.EOL, bulkdata, kind.EOL),
		},
		{
			name:           "Test empty array",
			message:        Array(),
			expectedOutput: fmt.Sprintf("*0\r\n"),
		},
		{
			name: "Test [hello, world]",
			message: Array(
				BulkString(strings.NewReader("hello"), 5),
				BulkString(strings.NewReader("world"), 5),
			),
			expectedOutput: "*2\r\n$5\r\nhello\r\n$5\r\nworld\r\n",
		},
		{
			name:           "[1, 2, 3]",
			expectedOutput: "*3\r\n:1\r\n:2\r\n:3\r\n",
			message: Array(
				Int(1),
				Int(2),
				Int(3),
			),
		},

		{
			name:           "%2\r\n+first\r\n:1\r\n+second\r\n:2\r\n",
			expectedOutput: "%2\r\n+first\r\n:1\r\n+second\r\n:2\r\n",
			message: Map(
				SimpleString("first"), Int(1),
				SimpleString("second"), Int(2),
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoder := Encoder{}
			buffer := &bytes.Buffer{}

			total, err := encoder.Encode(test.message, buffer)
			assert.NilError(t, err)
			assert.Equal(t, test.expectedOutput, buffer.String())
			assert.Equal(t, total, buffer.Len())
		})
	}
}

// TestEncode_ErrorMessage tests the Encode function for error type messages
func TestEncode_ErrorMessage(t *testing.T) {
	encoder := Encoder{}
	buffer := &bytes.Buffer{}

	message := Error("this is an error")

	_, err := encoder.Encode(message, buffer)
	if err == nil {
		t.Fatal("expected error but got none")
	}

	expectedError := "empty Error field for Error type message"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("expected error message to contain %q, got %v", expectedError, err)
	}
}

// TestWriteBulk tests the writeBulk method
func TestWriteBulk(t *testing.T) {
	encoder := Encoder{ChunkSize: 4}
	buffer := &bytes.Buffer{}

	s := "this is a bulk data test"
	message := BulkString(strings.NewReader(s), int64(len(s)))

	total, err := encoder.writeBulk(message, buffer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedOutput := s + kind.EOL
	if buffer.String() != expectedOutput {
		t.Errorf("expected output %q, got %q", expectedOutput, buffer.String())
	}

	if total != len(expectedOutput) {
		t.Errorf("expected total bytes written %d, got %d", len(expectedOutput), total)
	}
}

// TestEmptyBuffer tests the emptyBuffer helper function
func TestEmptyBuffer(t *testing.T) {
	tests := []struct {
		name      string
		chunkSize int64
		totalSize int64
		expected  int
	}{
		{"Valid Chunk Size", 4, 10, 4},
		{"Less than Chunk Size", 4, 3, 3},
		{"Default Buffer Size", 0, 10, 10},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buffer := emptyBuffer(test.chunkSize, test.totalSize)
			if len(buffer) != test.expected {
				t.Errorf("expected buffer length %d, got %d", test.expected, len(buffer))
			}
		})
	}
}
