# Retired: private transient writers for nfsd

This branch restores the pre-#191 model: each write OPEN gets its own
transient filehandle and staging, and each writer's CLOSE publishes its own
version. It works for the Linux kernel client and for libnfs and pynfs. It is
retired because it breaks Samba, which is how nfsd is reached in the initial
deployment.

## Why it does not work behind Samba

How Samba uses file descriptors:

- One smbd per SMB connection, and one kernel fd per SMB open, closed on
  SMB2 CLOSE. Samba does not pool or share fds between handles.
- Samba 4.15 and later (we run 4.17) open an existing file in two steps. It
  first opens the path `O_PATH`, which sends no NFS OPEN, then gets a real fd
  by opening `/proc/self/fd/N`. That reopen goes straight to the inode the
  first descriptor holds. It never looks the name up again. A file that does
  not exist yet is opened by name, so creates are not affected.
- All of Samba is one NFS client. Every smbd goes through the same kernel
  mount, and Linux picks the open-owner by credential, so with one local user
  everything also shares one open-owner.
- The share has `oplocks = no`, so Samba grants no leases. Windows has no
  deferred close and in practice no durable handles. An SMB handle closes
  when the application closes the file.

What goes wrong:

1. Samba cannot open an existing file for writing. The private-branch Samba
   run shows one LOOKUP, then three `CLAIM_FH` OPENs on the held handle, all
   `NFS4ERR_STALE`, and no pathname open. The three attempts are Linux's open
   retries. For an open by name the client drops the stale dentry after the
   first STALE, and the retry looks the name up again and sends `CLAIM_NULL`.
   Here the `O_PATH` descriptor still holds the dentry, so every retry lands
   on the same inode. Samba gets `ESTALE` and returns `ACCESS_DENIED`. That
   run had 87 such OPENs, and they account for most of the 23 extra
   nfsd-only Samba failures. Every in-place edit from Windows is affected:
   Notepad saves, appends, anything that opens an existing file for writing.
   New files and save-to-temp-then-rename still work. I think the old v4.0
   private model failed the same way: v4.0 sends `CLAIM_NULL`, but
   `nfs4_file_open` still rejects a reply whose inode differs from the
   dentry's.
2. It cannot be fixed by returning a private handle. Linux keeps the inode it
   already holds after a `CLAIM_FH` OPEN, whatever filehandle the reply
   names. Two fds on the same dentry share one inode and one page cache on
   the client, so the client cannot keep two private versions apart.
   Private-per-open only works when every open looks the name up again, and
   Samba never does.
3. SMB expects coherent handles: two handles on the same file see each
   other's writes at once. Shared staging per client gives that, since Samba
   is one client and its readers read the client's staging. Private writers
   do not.

What the shared model costs behind Samba: a file is published only when the
last SMB handle with write access to it closes, from any Windows user. Samba
users do not notice, because they read the unpublished staging. TernFS-side
readers (the cluster through the kmod) do, and the data stays only on the
nfsd host's staging disk until then. With oplocks off, only genuinely
long-lived write handles cause this: log files, applications that hold a data
file open, editors that write in place. Office-style saves publish on the
rename either way.

Bottom line: for a Samba-only deployment the private model is the wrong one.
It breaks in-place writes and drops the coherence Samba relies on. Keep shared
staging per client and fix progress separately: publish a snapshot whenever
any writer closes (or on fsync, or on a timer) and keep the shared staging
open for the writers that remain. This branch's change to always publish
through a fresh immutable candidate, so the live staging inode is never
linked, is the piece that makes that possible. The cost is re-uploading the
whole file on each close when a file has several writers.

## What the branch does

- A write OPEN by name constructs private staging and returns its transient
  filehandle. Writers on one client no longer join each other.
- A fresh writable `CLAIM_FH` gets `NFS4ERR_STALE`, so Linux retries by name.
  `NFS4ERR_INVAL` does not make Linux retry. A writable `CLAIM_FH` naming the
  client's own recoverable staging resumes that writer after a restart.
- A read `CLAIM_FH` reopens a valid immutable handle, and gets STALE only if
  a name this process remembered for it now holds a replacement.
- A fresh named OPEN never adopts an old writer by pathname.
- The first publication always goes through a fresh immutable candidate, so
  an unanswered link never exposes writable staging to a new reader. The
  original staging transient is scrapped separately.
- Startup aliases only the writer's own private handle, never the base an
  independent reader holds.
- `LocalTernVFS` keeps synthetic inode IDs and transient cookies in xattrs,
  read with `Lgetxattr` and only for regular files.

## Testing

All runs are against `b82538b5` or the uncommitted tree just before it, on
main `e8474d41`.

Passed:

- `go test ./nfsd/...`, plain and with `-race`.
- QEMU, Linux 6.1 client, local-root nfsd: all ten terntests mutation cases,
  including private writers and visible creation, and all 24 functional
  application workflows.
- Real cluster (`build/alpine`, ternrun): all 24 functional workflows,
  `TestTernPrivateMutableWriters`, `TestTernVisibleCreation`, and
  restart-mid-write, which recovered the open Linux writer and published
  129024 bytes.
- libnfs (`-short`).
- pynfs: 266 tests, no failures, with `OPEN2` and `OPEN7` added to
  `pynfs_unsupported.txt`. `OPEN7` expects a writable `CLAIM_FH` to succeed.
  `OPEN2` expects a second OPEN by name to advance the first open's stateid,
  which cannot hold when the second OPEN selects a different inode.
- Regression tests for the five problems found in review: read `CLAIM_FH`
  recovery after restart, fresh opens adopting old writers after restart,
  writable staging exposed by an unanswered link, startup redirecting an
  independent reader, and symlinks sharing an identity. Also the transient
  leak found while fixing them.

Failed or not run:

- Samba, before the review fixes, against main on the same cluster: main had
  86 cases matching the control share and no nfsd-only failures; this branch
  had 64 matching, 23 nfsd-only failures (byte-range locks, share modes,
  `read/access`, `create/brlocked`, `getinfo/granted`, two timestamp cases)
  and 2 control-only. After the review fixes only `smb2.read` was rerun, and
  it passes 5 of 5. The write suites were not rerun and should still fail as
  described above.
- The full real-cluster suite fails `TestReaddirPreparationWithinReservation`
  on its heap metric ("retained 0 bytes"). It passes run alone, on this
  branch and on main.
- `-race` on the real cluster reports a data race in `core/wyhash` through
  `ReqTimeouts.NextNow`. Main has it too.
- `ci_nfs` was not run; it needs KVM.
- The eggsfs-nfs-test harness on r73 was not run.

Artifacts:

- `dist/ternfs-release-r1-gb82538b580ca_0.1.0-1_amd64.deb`, sha256
  `2868da12c09a34ae5bd58feb0484cc43c70e724a2fd20c208f7e3055cfa1f6c6`.
  `release.json` says `source_dirty: true` only because of untracked files
  in the checkout. Not uploaded.
- restech1 `67767d406e` pins the QA image to that release. Not pushed or
  deployed.
