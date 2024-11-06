# anarchoredis

Leaderless replication for Redis

This is inspired by:

1. The now [years-old Jepsen analysis of Redis](https://aphyr.com/posts/283-jepsen-redis)
2. The leaderless replication section from [Designing Data-Intesnive Applications]([url](https://learning.oreilly.com/library/view/designing-data-intensive-applications/9781491903063/ch05.html#sec_replication_topologies))

Given a set of redis nodes, no server side changes are needed to accomplish the replication task. It's simply a task of creating the appropriate client.


> In some leaderless implementations, the client directly sends its writes to several replicas, while in others, a coordinator node does this on behalf of the client. However, unlike a leader database, that coordinator does not enforce a particular ordering of writes. As we shall see, this difference in design has profound consequences for the way the database is used.

> To solve that problem, when a client reads from the database, it doesn’t just send its request to one replica: read requests are also sent to several nodes in parallel. The client may get different responses from different nodes; i.e., the up-to-date value from one node and a stale value from another. Version numbers are used to determine which value is newer (see “Detecting Concurrent Writes”).

> ### Read repair
> When a client makes a read from several nodes in parallel, it can detect any stale responses. For example, in Figure 5-10, user 2345 gets a version 6 value from replica 3 and a version 7 value from replicas 1 and 2. The client sees that replica 3 has a stale value and writes the newer value back to that replica. This approach works well for values that are frequently read.

> ### Anti-entropy process
> In addition, some datastores have a background process that constantly looks for differences in the data between replicas and copies any missing data from one replica to another. Unlike the replication log in leader-based replication, this anti-entropy process does not copy writes in any particular order, and there may be a significant delay before data is copied.


So at a high level this project is a redis client that:

1. makes read and write requests in parallel to all nodes in the cluster
2. adds a `anrcho_version:*` prefixed key for every key in the cluster which is updated in a redis transaction along with any write operation
3. after a successful operation, self-healing needs to determine the full state of the value at a given key (e.g. if the operation was an LPUSH, we need to read the entire list and ensure all the keys have an up to date reason.
4. create a scheduled self healer that periodically compares sets of keys to ensure they are up to date.
   
# Redis client for Go

[![build workflow](https://github.com/redis/go-redis/actions/workflows/build.yml/badge.svg)](https://github.com/redis/go-redis/actions)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/redis/go-redis/v9)](https://pkg.go.dev/github.com/redis/go-redis/v9?tab=doc)
[![Documentation](https://img.shields.io/badge/redis-documentation-informational)](https://redis.uptrace.dev/)
[![codecov](https://codecov.io/github/redis/go-redis/graph/badge.svg?token=tsrCZKuSSw)](https://codecov.io/github/redis/go-redis)
[![Chat](https://discordapp.com/api/guilds/752070105847955518/widget.png)](https://discord.gg/rWtp5Aj)

> go-redis is brought to you by :star: [**uptrace/uptrace**](https://github.com/uptrace/uptrace).
> Uptrace is an open-source APM tool that supports distributed tracing, metrics, and logs. You can
> use it to monitor applications and set up automatic alerts to receive notifications via email,
> Slack, Telegram, and others.
>
> See [OpenTelemetry](https://github.com/redis/go-redis/tree/master/example/otel) example which
> demonstrates how you can use Uptrace to monitor go-redis.

## How do I Redis?

[Learn for free at Redis University](https://university.redis.com/)

[Build faster with the Redis Launchpad](https://launchpad.redis.com/)

[Try the Redis Cloud](https://redis.com/try-free/)

[Dive in developer tutorials](https://developer.redis.com/)

[Join the Redis community](https://redis.com/community/)

[Work at Redis](https://redis.com/company/careers/jobs/)

## Documentation

- [English](https://redis.uptrace.dev)
- [简体中文](https://redis.uptrace.dev/zh/)

## Resources

- [Discussions](https://github.com/redis/go-redis/discussions)
- [Chat](https://discord.gg/rWtp5Aj)
- [Reference](https://pkg.go.dev/github.com/redis/go-redis/v9)
- [Examples](https://pkg.go.dev/github.com/redis/go-redis/v9#pkg-examples)

## Ecosystem

- [Redis Mock](https://github.com/go-redis/redismock)
- [Distributed Locks](https://github.com/bsm/redislock)
- [Redis Cache](https://github.com/go-redis/cache)
- [Rate limiting](https://github.com/go-redis/redis_rate)

This client also works with [Kvrocks](https://github.com/apache/incubator-kvrocks), a distributed
key value NoSQL database that uses RocksDB as storage engine and is compatible with Redis protocol.

## Features

- Redis commands except QUIT and SYNC.
- Automatic connection pooling.
- [Pub/Sub](https://redis.uptrace.dev/guide/go-redis-pubsub.html).
- [Pipelines and transactions](https://redis.uptrace.dev/guide/go-redis-pipelines.html).
- [Scripting](https://redis.uptrace.dev/guide/lua-scripting.html).
- [Redis Sentinel](https://redis.uptrace.dev/guide/go-redis-sentinel.html).
- [Redis Cluster](https://redis.uptrace.dev/guide/go-redis-cluster.html).
- [Redis Ring](https://redis.uptrace.dev/guide/ring.html).
- [Redis Performance Monitoring](https://redis.uptrace.dev/guide/redis-performance-monitoring.html).
- [Redis Probabilistic [RedisStack]](https://redis.io/docs/data-types/probabilistic/)

## Installation

go-redis supports 2 last Go versions and requires a Go version with
[modules](https://github.com/golang/go/wiki/Modules) support. So make sure to initialize a Go
module:

```shell
go mod init github.com/my/repo
```

Then install go-redis/**v9**:

```shell
go get github.com/redis/go-redis/v9
```

## Quickstart

```go
import (
    "context"
    "fmt"

    "github.com/redis/go-redis/v9"
)

var ctx = context.Background()

func ExampleClient() {
    rdb := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "", // no password set
        DB:       0,  // use default DB
    })

    err := rdb.Set(ctx, "key", "value", 0).Err()
    if err != nil {
        panic(err)
    }

    val, err := rdb.Get(ctx, "key").Result()
    if err != nil {
        panic(err)
    }
    fmt.Println("key", val)

    val2, err := rdb.Get(ctx, "key2").Result()
    if err == redis.Nil {
        fmt.Println("key2 does not exist")
    } else if err != nil {
        panic(err)
    } else {
        fmt.Println("key2", val2)
    }
    // Output: key value
    // key2 does not exist
}
```

The above can be modified to specify the version of the RESP protocol by adding the `protocol`
option to the `Options` struct:

```go
    rdb := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "", // no password set
        DB:       0,  // use default DB
        Protocol: 3, // specify 2 for RESP 2 or 3 for RESP 3
    })

```

### Connecting via a redis url

go-redis also supports connecting via the
[redis uri specification](https://github.com/redis/redis-specifications/tree/master/uri/redis.txt).
The example below demonstrates how the connection can easily be configured using a string, adhering
to this specification.

```go
import (
    "github.com/redis/go-redis/v9"
)

func ExampleClient() *redis.Client {
    url := "redis://user:password@localhost:6379/0?protocol=3"
    opts, err := redis.ParseURL(url)
    if err != nil {
        panic(err)
    }

    return redis.NewClient(opts)
}

```


### Advanced Configuration

go-redis supports extending the client identification phase to allow projects to send their own custom client identification.

#### Default Client Identification

By default, go-redis automatically sends the client library name and version during the connection process. This feature is available in redis-server as of version 7.2. As a result, the command is "fire and forget", meaning it should fail silently, in the case that the redis server does not support this feature.

#### Disabling Identity Verification

When connection identity verification is not required or needs to be explicitly disabled, a `DisableIndentity` configuration option exists. In V10 of this library, `DisableIndentity` will become `DisableIdentity` in order to fix the associated typo.

To disable verification, set the `DisableIndentity` option to `true` in the Redis client options:

```go
rdb := redis.NewClient(&redis.Options{
    Addr:            "localhost:6379",
    Password:        "",
    DB:              0,
    DisableIndentity: true, // Disable set-info on connect
})
```

#### Unstable RESP3 Structures for RediSearch Commands
When integrating Redis with application functionalities using RESP3, it's important to note that some response structures aren't final yet. This is especially true for more complex structures like search and query results. We recommend using RESP2 when using the search and query capabilities, but we plan to stabilize the RESP3-based API-s in the coming versions. You can find more guidance in the upcoming release notes.

## Contributing

Please see [out contributing guidelines](CONTRIBUTING.md) to help us improve this library!

## Look and feel

Some corner cases:

```go
// SET key value EX 10 NX
set, err := rdb.SetNX(ctx, "key", "value", 10*time.Second).Result()

// SET key value keepttl NX
set, err := rdb.SetNX(ctx, "key", "value", redis.KeepTTL).Result()

// SORT list LIMIT 0 2 ASC
vals, err := rdb.Sort(ctx, "list", &redis.Sort{Offset: 0, Count: 2, Order: "ASC"}).Result()

// ZRANGEBYSCORE zset -inf +inf WITHSCORES LIMIT 0 2
vals, err := rdb.ZRangeByScoreWithScores(ctx, "zset", &redis.ZRangeBy{
    Min: "-inf",
    Max: "+inf",
    Offset: 0,
    Count: 2,
}).Result()

// ZINTERSTORE out 2 zset1 zset2 WEIGHTS 2 3 AGGREGATE SUM
vals, err := rdb.ZInterStore(ctx, "out", &redis.ZStore{
    Keys: []string{"zset1", "zset2"},
    Weights: []int64{2, 3}
}).Result()

// EVAL "return {KEYS[1],ARGV[1]}" 1 "key" "hello"
vals, err := rdb.Eval(ctx, "return {KEYS[1],ARGV[1]}", []string{"key"}, "hello").Result()

// custom command
res, err := rdb.Do(ctx, "set", "key", "value").Result()
```

## Run the test

go-redis will start a redis-server and run the test cases.

The paths of redis-server bin file and redis config file are defined in `main_test.go`:

```go
var (
	redisServerBin, _  = filepath.Abs(filepath.Join("testdata", "redis", "src", "redis-server"))
	redisServerConf, _ = filepath.Abs(filepath.Join("testdata", "redis", "redis.conf"))
)
```

For local testing, you can change the variables to refer to your local files, or create a soft link
to the corresponding folder for redis-server and copy the config file to `testdata/redis/`:

```shell
ln -s /usr/bin/redis-server ./go-redis/testdata/redis/src
cp ./go-redis/testdata/redis.conf ./go-redis/testdata/redis/
```

Lastly, run:

```shell
go test
```

Another option is to run your specific tests with an already running redis. The example below, tests
against a redis running on port 9999.:

```shell
REDIS_PORT=9999 go test <your options>
```

## See also

- [Golang ORM](https://bun.uptrace.dev) for PostgreSQL, MySQL, MSSQL, and SQLite
- [Golang PostgreSQL](https://bun.uptrace.dev/postgres/)
- [Golang HTTP router](https://bunrouter.uptrace.dev/)
- [Golang ClickHouse ORM](https://github.com/uptrace/go-clickhouse)

## Contributors

Thanks to all the people who already contributed!

<a href="https://github.com/redis/go-redis/graphs/contributors">
  <img src="https://contributors-img.web.app/image?repo=redis/go-redis" />
</a>
