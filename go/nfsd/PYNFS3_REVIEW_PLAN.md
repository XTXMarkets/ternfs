# pynfs3 review, third pass

2026-09-11. pynfs3 at 91da0425. `go vet`, `go test ./nfsd/` and
`go test -race` pass. Nothing here changes code.

The earlier plan (per-owner index, one rule for unconfirmed owners,
abandonment returning whole states, `existingOpen`) has landed. This pass
is from scratch against the new code. pynfs4 and combined-stack items live
in `PYNFS4_REVIEW_PLAN.md` in the ternfs3 worktree.

## 1. Code

Nothing blocking.

- [ ] `addStateLocked` overwrites `owner.states[fileID]` silently. If an
      unconfirmed state for the file were ever present when a new state is
      created, the old one would stay in `os.states` with no index entry
      and `assertOpenStateIndex` would not see it until the next quiescent
      check. Panic or return an error if the slot is occupied; the
      invariant is the whole point of the index.
- [ ] `finishOpen` panics on a write reopen. It is unreachable today, but a
      panic in a client-driven path is worse than SERVERFAULT via `fail`.
      Return an error status instead.
- [ ] Stateid generation (`state.generation++` in reopen, confirm, close)
      does not skip zero. §9.1.3 applies to stateid seqids as well as owner
      seqids. Same one-line fix as the owner wrap.
- [ ] `Server.beforeStartOpen` exists only for one test. Gating the second
      OPEN on its `Lookup` through a blocking VFS wrapper gives the same
      ordering without a production hook.
- [ ] `purgeClient` still scans `os.owners` once per owner it removes. Fine
      at current scale; note it in a comment.

## 2. Tests

- [ ] The two concurrency tests verify at-most-once plus identical replay,
      which is the property that matters. They do not prove the second
      request waited on the owner lock, because both orderings yield a
      replay. Rename them to say what they check, or drop the
      "serialized" claim.
- [ ] CLOSE replay tests compare status only. `closeFileWithSeqStatusE`
      throws away the returned stateid. Compare the stateid as
      `TestOpenConfirmReplay` does.
- [ ] `TestCloseUnknownStateid` sends the all-zero anonymous stateid, so it
      hits the special-stateid path, not `unknownStateStatusLocked`. Use a
      non-zero unknown `other`.
- [ ] `TestOpenStateStoreExemptErrorDoesNotAdvance` covers two of the eight
      exempt codes.
- [ ] Unbounded channel waits in the concurrency and race tests hang on
      regression instead of failing. Wrap with a generous timeout. The
      `notifyNextGetMeta` counter reset depends on the first CLOSE having
      made exactly two `GetMeta` calls; comment or count per request.
- [ ] `TestRecoveredCloseOperationsAreStateidScoped` still uses a
      one-second `time.After` for a negative assertion.
- [ ] `TestReopenReusesState` names a variable `upgraded` for a read-to-read
      reopen.
- [ ] Missing protocol-level coverage: CLOSE with old and future
      generation; CLOSE of an already-closed stateid with a new seqid; OPEN
      returning PERM or ROFS advances the seqid; reopen after CLOSE gets a
      fresh `other`; post-restart CLOSE with the right file but wrong
      `other`; OPEN by the pre-restart owner after restart requires
      confirmation again; READ and WRITE with a purged stateid return
      EXPIRED; tombstone eviction downgrades EXPIRED to BAD_STATEID
      observably; OPEN with a revoked clientid returns STALE_CLIENTID.
- [ ] Duplication: three PUTFH+OPEN_CONFIRM builders (`confirmOpenState`,
      `openConfirmStatus`, `confirmClusterOpen`); RPC plumbing copied into
      `openFileForOwnerE` and `closeFileWithSeqStatusE`; the two
      unconfirmed-owner tests differ only in seqid; `TestSetclientidReboot`
      is a subset of `TestSetclientidRebootRemovesStagingFiles`;
      `TestSetclientidFlow` re-inlines `setupClientWithVerifier`;
      `TestOpenRflagsRequireConfirm` hand-builds the OPEN that
      `openFileForOwner` builds.
- [ ] The `epoch == 1` STALE_STATEID test cements the pynfs accommodation.
      The real case, a pre-restart stateid without a sidecar, returns
      BAD_STATEID and is untested.
- [ ] Store tests read `store.states`, `store.owners`, `store.expired` and
      friends where a behavioural assertion exists (READ with closed
      stateid gives BAD_STATEID; next OPEN by a retained owner does not set
      OPEN4_RESULT_CONFIRM; evicted tombstone gives BAD_STATEID instead of
      EXPIRED).
- [ ] `closeLocalStagingFiles` reaches into `LocalStagingStore` because the
      store has no `Close`. Add one.
- [ ] `open_state_test.go`: `ownerOperationResult` with a nil-able finish
      callback plus five adapters. Calling `startX` and `finishX` directly
      with one small `finish(op, fn)` helper reads more plainly.

## 3. Owner retention

Leave it unchanged here. Owners are freed only in `purgeClient`, so a live
client accumulates idle owners, but the fix needs a clock and a sweeper and
both live in pynfs4. Do not free owners at their last CLOSE: RFC 7530
§16.18.5 says to retain until a lease of inactivity, and immediate release
would put OPEN_CONFIRM back on most Linux opens. See
`PYNFS4_REVIEW_PLAN.md` section 2.

## Order

1. Trim tests without changing owner retention: the three
   PUTFH+OPEN_CONFIRM builders, the copied RPC plumbing, the two
   unconfirmed-owner tests, the two restart tests, the 323-line adapter
   layer in `open_state_test.go`, and restore the old `startTestServer`
   signature with a second helper to undo the mechanical churn. Keep the
   index assertion and the two guard tests.
2. Section 1 code items in one commit.
3. Section 2 coverage gaps.
4. Restack pynfs4, then do owner eviction there.
