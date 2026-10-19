# Functional filesystem tests

These tests exercise ordinary Linux application workflows against an existing
mounted filesystem. Each test accepts the test root as its only argument,
creates an isolated directory below it, and removes that directory on success.
A failed test preserves its directory and prints the path.

Run the complete suite from `go/nfsd/test`:

```sh
make test-functional FUNCTIONAL_TEST_ROOT=/mnt/qa-nfs/test
```

Run or trace one case directly:

```sh
./functional/0007-dd-direct.sh /mnt/qa-nfs/test
strace -ff -o /tmp/0007.trace \
  ./functional/0007-dd-direct.sh /mnt/qa-nfs/test
```

The suite checks supported workflows and explicitly rejected combinations.
Unexpected failures are reported rather than skipped.

Overwriting a file publishes a replacement inode on the last write CLOSE.
The open descriptor's FILEID still describes its original handle; a fresh
pathname lookup finds the replacement. The Vim cases set `backupskip` and
`backupcopy` explicitly so their write strategy does not depend on the test
root's path.

Writers of the same file on one client and daemon share staging. Other
clients, or writers through different daemons, have independent versions;
the last successful publication wins. These tests do not establish atomic
append across clients.

See the [test guide](../README.md) for other suites and
[Deployment](../../DEPLOYMENT.md) for the supported modes and known issues.

Cases 0018, 0019 and 0023 unlink from the client which holds the open
descriptor. Linux normally implements this with a `.nfs...` rename followed
by CLOSE and REMOVE, so these cases exercise that client workflow. Direct
REMOVE while another open handle remains usable is covered by the raw
protocol tests, including the TernFS-backed namespace tests.
Case 0018 allows up to five seconds for asynchronous `.nfs...` cleanup after
close. Other leftover names fail immediately; `.nfs...` names which remain
after the timeout also fail.

| Case | Workflow |
| --- | --- |
| `0001-shell-create.sh` | Create a file with shell redirection. |
| `0002-shell-append.sh` | Append to a file with shell redirection. |
| `0003-python-write.sh` | Create and write a file with Python. |
| `0004-vim-create.sh` | Create a file with Vim. |
| `0005-vim-edit.sh` | Edit an existing file with Vim. |
| `0006-dd-create.sh` | Create a file with buffered `dd`. |
| `0007-dd-direct.sh` | Create a file with `dd` and direct I/O. |
| `0008-python-named-temporary-file.sh` | Close and reopen a named temporary file, then remove it. |
| `0009-sed-in-place.sh` | Replace a file using `sed -i`. |
| `0010-rsync-replace.sh` | Replace a file using `rsync` without metadata preservation. |
| `0011-rsync-in-place.sh` | Update a file using `rsync --inplace` without metadata preservation. |
| `0012-sort-output-over-input.sh` | Use one file as both input and output to `sort`. |
| `0013-git-commit.sh` | Create two Git commits with mode tracking and shared permissions disabled and an exact `safe.directory`. |
| `0014-tar-extract.sh` | Extract files and a symlink without restoring owner, mode or timestamps. |
| `0015-gzip-round-trip.sh` | Compress and decompress a file in place. |
| `0016-copy-sparse.sh` | Copy a sparse file without preserving metadata. |
| `0017-find-delete.sh` | Delete a mixed tree with `find -depth -delete`. |
| `0018-python-temporary-file.sh` | Use Python `TemporaryFile`. |
| `0019-unlink-recreate.sh` | Unlink an open file, recreate its name, and keep writing. |
| `0020-rename-file-open.sh` | Rename a new file while its descriptor remains open. |
| `0021-log-rotation.sh` | Rename an open log, recreate its old name, and keep writing. |
| `0022-rename-open-across-directories.sh` | Attempt to move an open file into another directory. |
| `0023-read-unlinked-open-file.sh` | Read unwritten data through the descriptor of an unlinked file. |
| `0024-rename-over-open-destination.sh` | Rename over a destination which remains open. |
