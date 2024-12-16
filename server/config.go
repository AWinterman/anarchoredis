package server

import "github.com/alexflint/go-arg"

type Config struct {
	Address           string   `arg:"--address" env:"AR_LISTEN_ADDRESS" help:"address to listen on" default:"localhost:36379"`
	MaxSize           int64    `arg:"--proto-max-bulk-len" env:"AR_PROTO_MAX_BULK_LEN" help:"max length of bulk string" default:"0"`
	RedisServersAddrs []string `arg:"--redis-servers" env:"AR_REDIS_SERVERS" help:"redis servers to connect to"`
}

func (c *Config) getMaxSize() int64 {
	if c.MaxSize == 0 {
		return 512 * 1000000
	}
	return c.MaxSize
}

func (c *Config) Parse() error {
	if c == nil {
		c = &Config{}
	}

	err := arg.Parse(c)

	return err
}
