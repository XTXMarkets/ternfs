# Shard database snapshots

To create snapshot `42` on replica `0` of shard `7`:

```sh
terncli -registry registry-proxy:9090 shard-req \
    -shard 7 -replica 0 -kind SHARD_SNAPSHOT -req '{"SnapshotId":42}'
```

The registry or registry proxy selects the location. Omit `-replica` to send
to the current leader. Add `-yes` to skip confirmation.

The snapshot is a RocksDB checkpoint in `snapshot-42` under the selected
replica's data directory. It contains that replica's local state, which can
lag behind the leader. It does not create snapshots on the other replicas.

The Go client discovers and refreshes local leader and follower addresses.
Use `Client.ShardReplicaRequest` with `msgs.MakeShardReplicaId(shard, replica)`
to target a replica. Retries use that replica's latest cached addresses and
never fall back to the leader. Unknown or unavailable replicas time out
according to the client's shard timeout settings; the CLI exposes these as
global `-shard-*-timeout` flags.

Replica discovery requires an updated registry or registry proxy. An updated
proxy uses the existing `AllShards` request upstream, so its upstream registry
does not need the new discovery request. New clients retain leader discovery
against older registries and proxies. The selected shard server must support
follower snapshots.
