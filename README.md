# anarchoredis

Strongly Consistent Redis

1. Implement the Redis wire protocol
2. Understand which keys can be modified by a given command
3. Anarchoredis has a single write leader.
4. The write leader holds a mutually exclusive lock on each key modified by
   a command. A request must obtain the lock before the key can be written to.
5. Anarchoredis subscribes to the write leaders replication stream (using `PSYNC` and `REPLCONF`)
6. When a command is received on the replication stream, the write leader
   writes the command to a replicated log (e.g. Raft) and releases the lock on the key.
7. When the lock is released, the write leader responds to the client.

This is inspired by:

1. The now [years-old Jepsen analysis of Redis](https://aphyr.com/posts/283-jepsen-redis)
2. The leaderless replication section from [Designing Data-Intesnive Applications]([url](https://learning.oreilly.com/library/view/designing-data-intensive-applications/9781491903063/ch05.html#sec_replication_topologies))
3. [MemoryDB](https://www.amazon.science/publications/amazon-memorydb-a-fast-and-durable-memory-first-cloud-database)

I started out wanting to write a leaderless replication proxy for redis, but the presence of non-deterministic 
operations, like `SPOP` makes such a client undesirable, and such a proxy turned out to be relatively complicated 
anyhow.

Then I read the MemoryDB paper, linked. 

MemoryDB keeps a replicated log of all updates. Writes are acknowledged once a corresponding update has been written 
to the replicated log. Reads can be served with eventual consistency from any replica, or with strong consistency 
from the leader.

MemoryDB navigates leader election with the replicated log as well, although the details of how this is accomplished 
are not published.

## Tasks

- [x] Proxy redis commands
- [x] Delay write acknowledgement until replication + Kafka
- [ ] leader election
- [ ] All redis commands
  - [x] String commands
  - [ ] ...

