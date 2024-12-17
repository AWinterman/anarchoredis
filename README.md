# anarchoredis

Leaderless replication for Redis

This is inspired by:

1. The now [years-old Jepsen analysis of Redis](https://aphyr.com/posts/283-jepsen-redis)
2. The leaderless replication section from [Designing Data-Intesnive Applications]([url](https://learning.oreilly.com/library/view/designing-data-intensive-applications/9781491903063/ch05.html#sec_replication_topologies))
3. [MemoryDB](https://www.amazon.science/publications/amazon-memorydb-a-fast-and-durable-memory-first-cloud-database)

I started out wanting to write a leaderless replication proxy for redis, but the presence of non-deterministic 
operations, like `SPOP` makes such a client undesirable, and such a proxy turned out to be relatively complicated 
anyhow.

Then I read the MemoryDB paper, linked, and though that was more appealing from a consistency perspective.


## Tasks

- [x] Proxy redis commands
- [x] Delay write acknowledgement until replication + Kafka
- [ ] leader election
- [ ] All redis commands
  - [x] String commands
  - ...

