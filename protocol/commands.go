package protocol

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"slices"
	"strings"

	"github.com/awinterman/anarchoredis/protocol/message"
)

type Command struct {
	// Name is the name of the command
	Name string

	// Args are all the strings in the command after the name.
	Args iter.Seq2[Message, error]

	Database string

	// Message is the original message
	Message message.Message
}

var commandsWithoutKey = map[string]bool{"FLUSHALL": true, "FLUSHDB": true, "SELECT": true, "FUNCTION": true,
	"CLIENT": true, "CLUSTER": true, "ACL": true, "COMMAND": true, "CONFIG": true, "PING": true}
var commandsWithSubOp = map[string]bool{"BITOP": true, "FUNCTION": true, "SCRIPT": true, "CLIENT": true,
	"CLUSTER": true, "ACL": true, "COMMAND": true, "CONFIG": true}

// ErrInvalidCommand is returned when a command is invalid
var ErrInvalidCommand = errors.New("invalid command")
var ErrNotImplemented = errors.New("not implemented")

// Cmd reads a command from the msg
//
// every command starts as a
// Clients send commands to a Redis server as an array of bulk strings. The
// first (and sometimes also the second) bulk string in the array is the
// command's name. Subsequent elements of the array are the arguments for
// the command.
//
// The server replies with a RESP type. The reply's type is
// determined by the command's implementation and possibly by the client's
// protocol version.
//
// at time of writing, this is exclusively parsing the aof file
func Cmd(msg message.Message) (*Command, error) {
	cmd := &Command{}
	cmd.Message = msg

	if msg.Kind != Array {
		return nil, fmt.Errorf("%w; expected array got %s", ErrInvalidCommand, msg.Kind)
	}

	var i int
	for arg, err := range msg.Seq {
		i++
		if err != nil {
			return nil, err
		}
		if arg.Kind != BulkString {
			return nil, fmt.Errorf("%w; expected Bulk for %d-th element of message, string got %s",
				ErrInvalidCommand, i, arg.Kind)
		}
		all, err := arg.ReadAll()
		if err != nil {
			return nil, err
		}

		if i == 0 {
			if cmd.Name = strings.ToUpper(all); cmd.Name == "" {
				return nil, fmt.Errorf("%w; expected non-empty string for command name", ErrInvalidCommand)
			}
			if commandsWithoutKey[cmd.Name] {
				if msg.RunLength > 2 {
					return nil, fmt.Errorf("%w; expected at least two elements for command %s got %d", ErrInvalidCommand,
						cmd.Name, msg.RunLength)
				}
			}
			cmd.Args = msg.Seq
		}

		if !commandsWithSubOp[cmd.Name] {
			cmd.Args = msg.Seq
			return cmd, nil
		}
		if i == 1 {
			if msg.RunLength < 3 {
				return nil, fmt.Errorf("%w; expected at least three elements for command %s got %d", ErrInvalidCommand,
					cmd.Name, msg.RunLength)
			}
			readAll, err := arg.ReadAll()
			if err != nil {
				return nil, err
			}
			cmd.Name = cmd.Name + " " + strings.ToUpper(readAll)
			cmd.Args = msg.Seq
		}
	}

	return cmd, nil
}

func firstN(n int) func(args iter.Seq2[Message, error], size int) ([]string, error) {
	return func(args iter.Seq2[Message, error], size int) ([]string, error) {
		if size < n {
			return []string{}, fmt.Errorf("%w; expected at least %d argument for firstArgKey", ErrInvalidCommand, n)
		}
		var keys []string
		var err error
		args(func(m Message, err error) bool {
			var all string
			all, err = m.ReadAll()
			if err != nil {
				return false
			}
			keys = append(keys, all)
			return len(keys) <= n
		})
		return keys, err
	}
}

var firstArgKeyFunc = firstN(1)

// oddIndices returns all the odd indices of args
func oddIndices(args iter.Seq2[Message, error], size int) ([]string, error) {
	if size%2 == 1 {
		return nil, fmt.Errorf("%w: expected an even number of arguments", ErrInvalidCommand)
	}
	// returns all the odd indices of args

	var keys []string
	var err error

	args(func(m Message, err error) bool {
		var all string
		all, err = m.ReadAll()
		if err != nil {
			return false
		}
	})

	return keys, nil
}

// allArgs processes a sequence of Messages and returns a slice of strings extracted from each Message using ReadAll.
// If an error occurs while reading a Message, it terminates processing and returns the error encountered.
func allArgs(args iter.Seq2[Message, error], _ int) ([]string, error) {
	var keys []string
	var err error
	args(func(m Message, err error) bool {
		var all string
		all, err = m.ReadAll()
		if err != nil {
			return false
		}
		keys = append(keys, all)
		return true
	})
	return keys, err
}

type CommandSpecification struct {
	Keys       func(iter.Seq2[Message, error], int) ([]string, error)
	Categories []string
}

func noKeysFunc(iter.Seq2[Message, error], int) ([]string, error) {
	return nil, nil
}

var cmdSpec = map[string]CommandSpecification{
	// connection
	"SELECT": {noKeysFunc, []string{"fast", "connection"}},

	//keyspace
	"UNLINK":   {allArgs, []string{"keyspace", "write", "fast"}},
	"FLUSHALL": {noKeysFunc, []string{"keyspace", "write", "slow", "dangerous"}},

	// strings
	"APPEND":      {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"DECR":        {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"DECRBY":      {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"GET":         {firstArgKeyFunc, []string{"read", "string", "fast"}},
	"GETDEL":      {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"GETEX":       {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"GETRANGE":    {firstArgKeyFunc, []string{"read", "string", "slow"}},
	"GETSET":      {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"INCR":        {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"INCRBY":      {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"INCRBYFLOAT": {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"LCS": {func(args []string) ([]string, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("%w: expected at least two arguments", ErrInvalidCommand)
		}
		return args[0:2], nil
	}, []string{"read", "string", "slow"}},
	// MGET key [key ...]
	"MGET": {allArgs, []string{"read", "string", "fast"}},
	//MSET key value [key value ...]
	"MSET":     {oddIndices, []string{"write", "string", "fast"}},
	"MSETNX":   {oddIndices, []string{"write", "string", "fast"}},
	"SET":      {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"SETRANGE": {firstArgKeyFunc, []string{"write", "string", "fast"}},
	"STRLEN":   {firstArgKeyFunc, []string{"read", "string", "fast"}},

	// sets
	"SADD": {firstArgKeyFunc, []string{"write", "set", "fast"}},
	"SREM": {firstArgKeyFunc, []string{"write", "set", "fast"}},

	// sorted sets
	"ZADD": {firstArgKeyFunc, []string{"write", "sortedset", "fast"}},
}

// Keys returns the keys affected by the command.
func (cmd *Command) Keys() ([]string, error) {
	specification, ok := cmdSpec[cmd.Name]
	if !ok {
		return nil, fmt.Errorf("%w: %s %w", ErrInvalidCommand, cmd.Name, ErrNotImplemented)
	}
	return specification.Keys(cmd.Args, int(cmd.Message.RunLength))
}

// IsWrite says whether the command would result in a write if executed
func (cmd *Command) IsWrite() bool {
	specification := cmdSpec[cmd.Name]
	return slices.Contains[[]string, string](specification.Categories, "write")

}
