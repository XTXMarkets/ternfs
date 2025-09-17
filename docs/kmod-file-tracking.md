<!--
Copyright 2025 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

TernFS files are immutable: they are created once and never modified. This presents a challenge when writing a kernel module for it, since the VFS API very much assumes that files can be modified.

So our high level strategy is: allow users to open a file for writing, and keep that file transient (i.e. not visible in the directory tree) until we declare it "done", and do not allow modifications after that.

The main problem in implementing the strategy above is when to declare the file "done". An attractive answer is "when the file is closed". However one problem with that answer is that it's not clear when files are "consciously" closed through through `close()`, and when they are closed because the process is winding down and all its open FDs are being closed.

The relevant VFS interface is `flush` in `struct file_operations`: exactly the same function gets called in the two situations above.

If we just blindly declare the file done when `flush` is called, we're going to get a ton of false positives. Consider the classic `fork` + `execve` pattern:

* An TernFS file is opened for writing by process `A`;
* `A` unrelatedly (perhaps in another thread) forks (maybe to run another process through `execve`) to process `B`;
* `B` inherits all FDs of `A`, including the open TernFS file;
* `B` terminates before `A` has finished writing the TernFS file it opened;
* The file is prematurely declared "done" and `A` can't finish writing it to completion.

So we need a better way to recognize when a `flush` is intentional, so to speak. We achieve this as follows:

* When a file gets created, we [attach to it a reference to the process that originated the syscall](https://internal-repo/blob/3ffe0a43efa6a604018d6a357e16fd05893f7797/kmod/inode.c#L206). Note that we use `current->group_leader` rather than `current` to get the _process_, not the _thread_, that the syscall originates from.
* We also [attach the `struct mm_struct`](https://internal-repo/blob/3ffe0a43efa6a604018d6a357e16fd05893f7797/kmod/inode.c#L207) of the process to the file.
* When flushing:
    - If the flush does not originate from the same process that originated the file creation, [we do not declare the file done](https://internal-repo/blob/3ffe0a43efa6a604018d6a357e16fd05893f7797/kmod/file.c#L802);
    - If the flush is in the same process, but the `struct mm_struct` of the process has already been torn down, then [interrupt the creation of the file altogether](https://internal-repo/blob/3ffe0a43efa6a604018d6a357e16fd05893f7797/kmod/file.c#L812). We do this since if the `struct mm_struct` of the process is done it means that we're in the middle of tearing the process down, which in turn means that no user requested for the file to be closed explicitly (i.e. the process has crashed before the file got closed). For evidence of this, note how [`exit_mm()` is called before `exit_files`](https://elixir.bootlin.com/linux/v5.4.249/source/kernel/exit.c#L845).

The above is quite dirty, but seems to be pretty solid[^fuse]. However trouble arises if files are created from inside the kernel, which is exactly what happens with NFS, which is what prompted me to write down this explanation. We'll have to do something else for NFS to work.

[^fuse]: Note that the FUSE implementation is _not_ as solid, given that we don't have access to the internals that we have access to in the kernel module.

Also note that we want to keep `struct mm_struct` around anyway to increase `MM_FILEPAGES` when we allocate new pages to write files. But that is more of a nice to have than a strict requirement.
