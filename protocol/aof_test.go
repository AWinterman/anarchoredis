package protocol

import (
	"bufio"
	"errors"
	"io"
	"os"
	"testing"
)

func TestAOFRead(t *testing.T) {
	file, err := os.Open("./appendonly.aof.1.incr.aof")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rd := bufio.NewReadWriter(bufio.NewReader(file), nil)

	var msg *Message
	for err == nil {
		msg, err = Read(rd)
		if err != nil {
			t.Log(err)
			break
		}

		command, rerr := msg.Cmd()
		if rerr != nil {
			t.Errorf("error parsing %v %T", rerr, rerr)
			err = rerr
		}

		keys, rerr := command.Keys()
		if rerr != nil {
			t.Errorf("error parsing %v %T", rerr, rerr)
			err = rerr
		}
		t.Logf("name => %q; keys => %v; db => %q", command.Name, keys, command.Database)
	}

	if !errors.Is(err, io.EOF) {
		t.Errorf("error at end of %v %T", err, err)
	}
}
