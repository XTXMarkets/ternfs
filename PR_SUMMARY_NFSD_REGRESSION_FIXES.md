nfsd: preserve staged writes across publication retries and restart

A backend timeout during CLOSE could discard an unpublished save. The Linux
client ignores a failed CLOSE's status, so the application could see success
while the pathname still held the previous version. CLOSE and OPEN_DOWNGRADE
now retain staging and return DELAY while publication is unresolved.

Each staging file records its publication candidate and data version. Retries
resolve that candidate before publishing through a fresh transient, which is
checkpointed before submission. REMOVE and RENAME fence pending candidates
before changing their names. Finalization checks the data version and stops
new writes under one lock, so a concurrent stable write cannot succeed and
then be discarded.

Operation errors remain replayable without retiring the session. COMMIT is
excluded from both recovery gates, preserving sessions for compounds such as
COMMIT followed by CLOSE. Recovered writers remain their client's read view.
Staging ownership follows the surviving writer or reader, and READ checks
staging again when a concurrent OPEN changes its source.

The sidecar format is NFS9. Older staging must be drained or recovered with a
compatible binary before upgrading.

Validation: `go test ./nfsd/...`, race tests for server, staging and openstate,
and `go vet ./nfsd/...` passed. Regression coverage includes failed candidate
checkpoints across restart, writes racing publication and clean or
timestamp-only CLOSE, and reads racing an OPEN upgrade. No live-cluster or
mounted-client suite was run.
