// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description:

// Package replication:
package replication

/* REPLCONF <option> <value> <option> <value> ...
 * This command is used by a replica in order to configure the replication
 * process before starting it with the SYNC command.
 * This command is also used by a master in order to get the replication
 * offset from a replica.
 *
 * Currently Redis support these options:
 *
 * - listening-port <port>
 * - ip-address <ip>
 * What is the listening ip and port of the Replica redis instance, so that
 * the master can accurately lists replicas and their listening ports in the
 * INFO output.
 *
 * - capa <eof|psync2>
 * What is the capabilities of this instance.
 * eof: supports EOF-style RDB transfer for diskless replication.
 * psync2: supports PSYNC v2, so understands +CONTINUE <new repl ID>.
 *
 * - ack <offset> [fack <aofofs>]
 * Replica informs the master the amount of replication stream that it
 * processed so far, and optionally the replication offset fsynced to the AOF file.
 * This special pattern doesn't reply to the caller.
 *
 * - getack <dummy>
 * Unlike other subcommands, this is used by master to get the replication
 * offset from a replica.
 *
 * - rdb-only <0|1>
 * Only wants RDB snapshot without replication buffer.
 *
 * - rdb-filter-only <include-filters>
 * Define "include" filters for the RDB snapshot. Currently we only support
 * a single include filter: "functions". Passing an empty string "" will
 * result in an empty RDB. */
