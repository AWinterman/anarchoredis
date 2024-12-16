package kafka

// the producer reads an AOF file and sends the commands to the kafka topic

import (
	"bufio"
	"bytes"
	"context"

	"github.com/awinterman/anarchoredis/protocol"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer reads the AOF file and sends the commands to the kafka topic
type Producer struct {
	ReadFunc func() (*protocol.Message, error)
	Topic    string
	Client   *kgo.Client
}

func (p *Producer) Run(ctx context.Context) error {
	var currentDB string
	var b = bytes.NewBuffer(make([]byte, 0, 5000))
	for ctx.Err() == nil {
		msg, err := p.ReadFunc()
		if err != nil {
			return err
		}

		cmd := &protocol.Command{}
		err = cmd.Parse(msg)
		if err != nil {
			return err
		}

		if cmd.Database != "" {
			currentDB = cmd.Database
		} else {
			cmd.Database = currentDB
		}

		w := bufio.NewWriter(b)
		_, err = protocol.Write(w, msg)

		// todo: make this batch
		results := p.Client.ProduceSync(ctx, &kgo.Record{
			Key:     []byte(cmd.Database),
			Value:   b.Bytes(),
			Headers: []kgo.RecordHeader{},
			Topic:   p.Topic,
		})

		for i := range results {
			if results[i].Err != nil {
				return results[i].Err
			}
		}
	}

	return ctx.Err()
}
