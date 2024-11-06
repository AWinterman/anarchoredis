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
   
