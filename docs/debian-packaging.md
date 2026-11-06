# Debian Packaging

TernFS userspace release packages are immutable and co-installable. Package
installation is separate from selecting and restarting a service, so different
shard, CDC, or block-service instances on one host can run different releases.

Users must implement their own deployment automation. The requirements below
describe how that automation should manage these packages.

## Package Contract

A userspace release package:

- is named `ternfs-release-r<build>-g<commit>`;
- installs under `/usr/lib/ternfs/releases/<release-id>/`;
- contains stripped binaries and a checksummed `release.json`;
- contains no environment configuration or credentials;
- contains no systemd units or maintainer scripts;
- does not enable, stop, start, or restart services;
- can be installed alongside every other TernFS release package.

Branches are build inputs, not deployment identities. Resolve a branch to a
commit before building and deploy the resulting immutable release ID.

Each release package has a matching `ternfs-release-r<build>-g<commit>-dbgsym`
debug symbol package, following Ubuntu's automatic debug symbol package
conventions. It depends on the exact release package version and installs one
`/usr/lib/debug/.build-id/<xx>/<rest>.debug` file per binary, where gdb,
elfutils, systemd-coredump and debuginfod look for it. Its `Build-Ids` field
lists the GNU build IDs of the binaries.

## Building

Build TernFS first, then package the completed output directory:

```bash
./build.sh alpine --build-number 1842
./packaging/build_release_deb.py \
    --build-number 1842 \
    --build-dir build/alpine \
    --output-dir dist
```

`build.sh --build-number` stamps the build number into the GNU build ID of
every binary. Use a distinct build number for every build you package, and give
the same number to `packaging/build_release_deb.py --build-number`. Without
the stamp, a binary that a change does not touch is byte-identical across
builds and keeps its build ID. The debug symbol packages of those builds then
contain the same files, and cannot be installed together. The release packages
themselves stay co-installable either way.

The source checkout must be clean. Every release binary must have a GNU build
ID. Stripping uses `objcopy` from binutils. The output is similar to:

```text
dist/ternfs-release-r1842-g0123456789ab_0.1.0-1_amd64.deb
dist/ternfs-release-r1842-g0123456789ab-dbgsym_0.1.0-1_amd64.ddeb
```

The package name identifies the build and source commit. The Debian version
is `<project-version>-1`, and `release.json` records the full source revision.

To distribute packages through apt, you should configure repository
publication, signing, and Debian distribution metadata. Publish the generated
packages without modifying their contents. The debug package uses Ubuntu's
`.ddeb` extension, so the repository must accept `.ddeb` files, usually in a
separate debug component or repository. Install it with apt or `dpkg -i` only
on hosts where you need symbols.

## Release Selection

Your deployment automation should separate package installation from release
activation and select one exact release ID for each service instance. It should:

1. resolve a source branch or tag to a commit and build one release package;
2. publish that package without changing it;
3. install it on the hosts that may run it, without restarting anything;
4. map each service instance to an exact release ID;
5. restart only the selected instances, with quorum-aware health checks; and
6. roll back by restoring the previous release ID and restarting the same
   instances.

The mapping key must identify a process, not only a host. For example, shard
`42:1`, CDC replica `2`, and the block service on a host can independently
select releases. A default release plus sparse per-instance overrides preserves
canaries without creating branch-specific package layouts.

Branches are convenient inputs for CI, but are mutable. A desired-state file
may refer to a branch for operator convenience only if the deployment
controller resolves it to a release ID before changing any host. Generated
systemd units and deployment audit records must contain the resolved release
ID.

## Deployment Requirements

You should implement a deployment integration that manages:

- apt repository publication, signing, and retention;
- host, location, shard, replica, and block-service inventory;
- the default release and canary overrides;
- service arguments, endpoints, credentials, users, and alert integration;
- quorum-aware rollout order and approval policy; and
- kernel-client rollout cohorts and exact version pins.

Your runtime integration should support release selection, systemd unit
generation, status reporting, and inactive-release cleanup. It should use the
desired state from your inventory. Ensure that installing or upgrading runtime
or release packages does not activate, stop, or restart TernFS services.

## Kernel Client Boundary

The `ternfs-client-dkms` package contains buildable kernel module source,
generated source dependencies, revision metadata, and DKMS control files. Files
copied from `go/core/crc32c` and `go/core/rs` are source dependencies and belong
in that package.

Mount endpoints, sysctls, systemd units, and enablement policy are deployment
configuration. You should supply them separately, optionally in a
site-specific package that depends on `ternfs-client-dkms`.

Only one TernFS kernel module version can be loaded on a host. Kernel-client
canaries are therefore host cohorts pinned to exact package versions, not
per-process overrides. Configuration-package installation must not unmount,
reload the module, or restart the mount.
