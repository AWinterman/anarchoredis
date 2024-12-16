package kafka

// the producer reads an AOF file and sends the commands to the kafka topic

import (
	"bufio"
	"context"
	"errors"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	Topic  string
	Client *kgo.Client
	Writer *bufio.Writer
}

func (c *Consumer) Run(ctx context.Context, client *kgo.Client) error {
	for ctx.Err() == nil {
		fetches := c.Client.PollFetches(ctx)
		var err error
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				return errors.Join(err, e.Err)
			}
		}

		// We can iterate through a record iterator...
		iter := fetches.RecordIter()
		for !iter.Done() {
			n := iter.Next()
			_, err := c.Writer.Write(n.Value)
			if err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}
