package protocol

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type Command struct {
	// Name is the name of the command
	Name string

	// Args are all the strings in the command after the name.
	Args []string

	Database string

	// Message is the original message
	Message *Message
}

var commandsWithoutKey = map[string]bool{"FLUSHALL": true, "FLUSHDB": true, "SELECT": true, "FUNCTION": true,
	"CLIENT": true, "CLUSTER": true, "ACL": true, "COMMAND": true, "CONFIG": true, "PING": true}
var commandsWithSubOp = map[string]bool{"BITOP": true, "FUNCTION": true, "SCRIPT": true, "CLIENT": true,
	"CLUSTER": true, "ACL": true, "COMMAND": true, "CONFIG": true}

// ErrInvalidCommand is returned when a command is invalid
var ErrInvalidCommand = errors.New("invalid command")

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
func (msg *Message) Cmd() (*Command, error) {
	cmd := &Command{}
	cmd.Message = msg

	if msg.Indicator != Array {
		return nil, fmt.Errorf("%w; expected array got %s", ErrInvalidCommand, msg.Indicator)
	}

	for i := 0; i < len(msg.Array); i++ {
		if msg.Array[i].Indicator != BulkString {
			return nil, fmt.Errorf("%w; expected BulkString for %d-th element of message, string got %s",
				ErrInvalidCommand, i, msg.Array[i].Indicator)
		}
	}

	if cmd.Name = strings.ToUpper(msg.Array[0].Str); cmd.Name == "" {
		return nil, fmt.Errorf("%w; expected non-empty string for command name", ErrInvalidCommand)
	}

	var startIndex = 1

	switch {
	case commandsWithSubOp[cmd.Name]:
		if len(msg.Array) < 3 {
			return nil, fmt.Errorf("%w; expected at least three elements for command %s got %d", ErrInvalidCommand,
				cmd.Name, len(msg.Array))
		}
		cmd.Name = cmd.Name + " " + strings.ToUpper(msg.Array[1].Str)
		startIndex = 2
	case !commandsWithoutKey[cmd.Name]:
		if len(msg.Array) < 2 {
			return nil, fmt.Errorf("%w; expected at least two elements for command %s got %d", ErrInvalidCommand,
				cmd.Name, len(msg.Array))
		}

	}

	for i := startIndex; i < len(msg.Array); i++ {
		cmd.Args = append(cmd.Args, msg.Array[i].Str)
	}

	return cmd, nil
}

func firstArgKeyFunc(args []string) ([]string, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("%w: expected at least one arguments command", ErrInvalidCommand)
	}
	return args[0:1], nil
}

// oddIndices returns all the odd indices of args
func oddIndices(args []string) ([]string, error) {
	if len(args)%2 == 1 {
		return nil, fmt.Errorf("%w: expected an even number of arguments", ErrInvalidCommand)
	}
	// returns all the odd indices of args

	var keys []string
	for i := 1; i < len(args); i += 2 {
		keys = append(keys, args[i])
	}
	return keys, nil
}

type CommandSpecification struct {
	Keys       func([]string) ([]string, error)
	Categories []string
}

func noKeysFunc([]string) ([]string, error) {
	return nil, nil

}

var cmdSpec = map[string]CommandSpecification{
	// connection
	"SELECT": {noKeysFunc, []string{"fast", "connection"}},

	//keyspace
	"UNLINK":   {func(args []string) ([]string, error) { return args, nil }, []string{"keyspace", "write", "fast"}},
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
	"MGET": {func(args []string) ([]string, error) { return args, nil }, []string{"read", "string", "fast"}},
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
		return nil, fmt.Errorf("%w: %s not supported", ErrInvalidCommand, cmd.Name)
	}
	return specification.Keys(cmd.Args)
}

// IsWrite says whether the command would result in a write if executed
func (cmd *Command) IsWrite() bool {
	specification := cmdSpec[cmd.Name]
	return slices.Contains[[]string, string](specification.Categories, "write")

}
