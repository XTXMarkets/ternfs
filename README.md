<!--
Copyright 2025 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

[![TernFS Logo](https://github.com/user-attachments/assets/2e895b48-1a61-4b9d-aad0-2fc03777e817 "TernFS Logo")](https://ternfs.com)

A distributed file system. For a high-level description of TernFS, see [the TernFS blog post on the XTX Markets Tech Blog](https://xtxmarkets.com/tech/2025-ternfs). This document provides a more bare-bones overview and an introduction to the codebase.

## Goals

The target use case for TernFS is the kind of machine learning we do at XTX: reading and writing large immutable files. By "immutable" we mean files that do not need modifying after they are first created. By "large" we mean that most of the storage space will be taken up by files bigger than a few MBs.

We don't expect new directories to be created often, and files (or directories) to be moved between directories often. In terms of numbers, we expect the upper bound for TernFS to roughly be the upper bound for the data we're planning for a single data center:

- 10EB of logical file storage (i.e. if you sum all file sizes = 10EB)
- 1 trillion files -- average ~10MB file size
- 100 billion directories -- average ~10 files per directory
- 1 million clients

We want to drive the filesystem with commodity hardware and Ethernet networking.

We want the system to be robust in various ways:

* Witnessing half-written files should be impossible -- a file is fully written by the writer or not readable by other clients
* Power loss or similar failure of storage or metadata nodes should not result in a corrupted filesystem (be it metadata or filesystem corruption)
* Corrupted reads due to hard drives bitrot should be exceedingly unlikely
* Data loss should be exceedingly unlikely, unless we suffer a datacenter-wide catastrophic event (fire, flooding, datacenter-wide vibration, etc.)
* The filesystem should keep working through maintenance or failure of metadata or storage nodes

We also want to be able to restore deleted files or directories, using a configurable "permanent deletion" policy.

Finally, we want to have the option to replicate TernFS to multiple regions, to remove any single data centre as a point of failure.

## Components

* **servers**
  * **web**
    * 1 logical instance
    * `ternweb`, go binary
    * TCP http server
    * stateless
    * serves web UI
  * **registry**
    * 1 logical instance
    * `ternregistry`, C++ binary
    * TCP bincode req/resp
    * UDP replication
    * stores metadata about a specific TernFS deployment
      * shard/cdc addresses
      * block services addressea and storage statistics
    * state persisted through RocksDB with 5-node distributed consensus through LogsDB
  * **filesystem data**
    * **metadata**
      * **shard**
        * 256 logical instances
        * `ternshard`, C++ binary
        * stores all metadata for the filesystem
          * file attributes (size, mtime, atime)
          * directory attributes (mtime)
          * directories listings (includes file/directory names)
          * file to blocks mapping
          * block service to file mapping
        * UDP bincode req/resp
        * state persisted through RocksDB with 5-node distributed consensus through LogsDB
        * communicates with registry to fetch block services, register itself, insert statistics
    * **CDC**
      * 1 logical instance
      * `terncdc`, C++ binary
      * coordinates actions which span multiple directories
        * create directory
        * remove directory
        * move file or directory between from one directory to the other
        * "Cross Directory Coordinator"
      * UDP bincode req/resp
      * very little state required
        * information about which transactions are currently being executed and which are queued (currently transactions are executed serially)
        * directory -> parent directory mapping to perform "no loops" checks
      * state persisted through RocksDB with 5-node distributed consensus through LogsDB
      * communicates with the shards to perform the cross-directory actions
      * communicates with registry to register itself, fetch shards, insert statistics
  * **block service**
    * up to 1 million logical instances
    * 1 logical instance = 1 disk
    * 1 physical instance handles all the disks in the server it's running on
    * `ternblocks`, Go binary (for now, might become C++ in the future if performance requires it)
    * stores "blocks", which are blobs of data which contain file contents
    * each file is split in many blocks stored on many block services (so that if up to 4 block services fail we can always recover files)
    * single instances are not redundant, the redundancy is handled by spreading files over many instances so that we can recover their contents
    * TCP bincode req/resp
    * extremely dumb, the only state is the blobs themselves
    * its entire job is efficiently streaming blobs of data from disks into TCP connections
    * communicates with registry to register itself and to update information about free space, number of blocks, etc.
* **clients**, these all talk to all of the servers
  * **cli**
    * `terncli`, Go binary
    * toolkit to perform various tasks, most notably
      * migrating contents of dead block services (`terncli migrate`)
      * moving around blocks so that files are stored efficiently (`terncli defrag`, currently WIP, see #50)
  * **kmod**
    * `ternfs.ko`, C Linux kernel module
    * kernel module implementing `mount -t eggsfs ...`
    * the most fun and pleasant part of the codebase
  * **FUSE**
    * `ternfuse`, Go FUSE implementation of a TernFS client
    * currently slower (especially when reading), and requires a BPF program to correctly detect file closes
  * **S3**
    * `terns3`, Go implementation of the S3 API
    * minimal example intended as a start point for a more serious implementation
* **daemons**, these also talk to all of the servers, and all live in `terngc`
  * **GC**
    * permanently deletes expired snapshots (i.e. deleted but not yet purged data)
    * cleans up all blocks for permanently deleted files
  * **scrubber**
    * goes around detecting and repairing bitrot
  * **migrator**
    * evacuates failed disks
* **additional tools**
  * `ternrun`, a tool to quickly spin up a TernFS instance
  * `terntests`, runs some integration tests

## Building

```
% ./build.sh alpine
```

Will build all the artifacts apart from the Kernel module. The output binaries will be in `build/alpine`. Things will be built in an Alpine Linux container, so that everything will be fully statically linked.

There's also `./build.sh ubuntu` which will do the same but in a Ubuntu container, and `./build.sh release` which will build outside docker, which means that you'll have to install some dependencies in the host machine. Both of these build options will have glibc as the only dynamically linked dependency.

We use the Ubuntu-built version in production, mostly due to jemalloc not playing well with Alpine. It should be easy to produce a performant Alpine build, we just never had the need so far.

## Testing

```
./ci.py --build --integration --short --docker
```

Will run the integration tests as CI would (inside the Ubuntu docker image). You can also run the tests outside docker by removing the `--docker` flag, but you might have to install some dependencies of the build process. These tests take roughly 20 minutes on our build server.

To work with the qemu kmod tests you'll first need to download the base Ubuntu image we use for testing:

```
% wget -P kmod 'https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img'
```

Then you can run the CI tests in kmod like so:

```
% ./ci.py --kmod --short --prepare-image=kmod/focal-server-cloudimg-amd64.img --leader-only
```

The tests redirect dmesg output to `kmod/dmesg`, event tracing output to `kmod/trace`, and the full test log to `kmod/test-out`.

You can also ssh into the qemu which is running the tests with

```
% ssh -p 2223 -i kmod/image-key fmazzol@localhost
```

Note that the kmod tests are very long (~1hr). Usually when developing the kernel module it's best to use `./kmod/restartsession.sh` to be dropped into qemu, and then run specific tests using `terntests`.

However when merging code modifying TernFS internals it's very important for the kmod tests to pass as well as the normal integration tests. This is due to the fact that the qemu environment is generally very different from the non-qemu env, which means that sometimes it'll surface issues that the non-qemu environment won't.

## Playing with a local TernFS instance

```
% cd go/ternrun
% go run . -data-dir <data-dir>
```

The above will run all the processes needed to run TernFS. This includes:

* 256 metadata shards;
* 1 cross directory coordinator (CDC)
* A bunch of block services (this is tunable with the `-flash-block-services`, `-hdd-block-services`, and `-failure-domains` flags)
* 1 registry instance

A multitude of directories to persist the whole thing will appear in `<data-dir>`. The filesystem will also be mounted using FUSE under `<data-dir>/fuse/mnt`.

Inspecting [`ternrun.go`](https://github.com/XTXMarkets/ternfs/blob/main/go/ternrun/ternrun.go) is a decent way to get familiar with the services required to run a TernFS cluster.

TODO add instruction on how to run a multi-datacenter setup.

TODO add migrator, gc, and so on to the above.

## Building the Kernel module

```
% cd kmod
% ./fetchlinux.sh # fetch linux sources
% (cd linux && make oldconfig && make prepare && make -j) # build Linux
% make KDIR=linux -j kmod
```

## VS Code

Most of the codebase is understandable by VS Code/LSP:

* Code in `go/` just works out of the box with the [Go extension](https://code.visualstudio.com/docs/languages/go). I (@bitonic) open a separate VS Code window which specifically has the `ternfs/go` directory open, since the Go extension doesn't seem to like working from a subdirectory.
* Code in `cpp/`:
    - Disable existing C++ integrations for VS Code (I don't remember which exact C++ extension caused me trouble -- something by Microsoft itself).
    - Install the [clangd extension](https://marketplace.visualstudio.com/items?itemName=llvm-vs-code-extensions.vscode-clangd).
    - Generate `compile_commands.json` with
        
        ```
        % cd cpp
        % ./build.py debug
        % cp ./build/debug/compile_commands.json .
        ```
        
        If you change the build process (i.e. if you change CMake files) you'll need to generate and copy `compile_commands.json` again.
* Code in `kmod/`:
    - [Build the module](#building-the-kernel-module).
    - Generate `compile_commands.json` with `./kmod/gen_compile_commands.sh`.
    - New files should work automatically, but if things stop working, just re-bulid and re-generate `compile_commands.json`.

## A note on naming

TernFS was originally called EggsFS internally. This name quickly proved to be very poor due to the phonetic similarity to XFS, another notable filesystem. Therefore the filesystem was renamed to TernFS before open sourcing. However the old name lingers on in certain areas of the system that would have been hard to change, such as metric names.

## Licensing

TernFS is [Free Software](https://www.gnu.org/philosophy/free-sw.en.html). The default license for TernFS is [GPL-2.0-or-later](LICENSE/GPL-2.0-or-later.txt).

The protocol definitions (`go/msgs/`), protocol generator (`go/bincodegen/`) and client library (`go/client/`, `go/core/`) are licensed under [Apache-2.0](LICENSES/Apache-2.0.txt) with the [LLVM-exception](LICENSES/LLVM-exception.txt). This license combination is both permissive (similar to MIT or BSD licenses) as well as compatible with both GPL license. We have done this to allow people to build their own proprietary client libraries while ensuring we can also freely incorporate them into the GPL v2 licensed Linux kernel.
