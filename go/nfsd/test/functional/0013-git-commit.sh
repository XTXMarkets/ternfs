#!/usr/bin/env bash

source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
functional_test_init "0013-git-commit" "$@"
functional_test_require git

repo="$FUNCTIONAL_TEST_DIR/repo"
mkdir -p "$repo/.git/objects" "$repo/.git/refs/heads" "$repo/.git/refs/tags"
printf 'ref: refs/heads/master\n' >"$repo/.git/HEAD"
printf '%s\n' \
	'[core]' \
	'	repositoryFormatVersion = 0' \
	'	fileMode = false' \
	'	bare = false' \
	'	logAllRefUpdates = true' \
	'	sharedRepository = 0' \
	'[user]' \
	'	name = NFS functional test' \
	'	email = nfs-functional@example.invalid' \
	'[commit]' \
	'	gpgSign = false' \
	>"$repo/.git/config"

global_config="$FUNCTIONAL_TEST_DIR/gitconfig"
printf '%s\n' \
	'[safe]' \
	"	directory = $repo" \
	>"$global_config"
export GIT_CONFIG_GLOBAL="$global_config"
export GIT_CONFIG_NOSYSTEM=1

run_git() {
	git -C "$repo" "$@"
}

printf 'first\n' >"$repo/data.txt"
run_git add data.txt
run_git commit -q -m first
printf 'second\n' >"$repo/data.txt"
run_git add data.txt
run_git commit -q -m second

if [[ $(run_git rev-list --count HEAD) != 2 ]]; then
	echo "Git repository does not contain two commits" >&2
	exit 1
fi
if [[ $(run_git show HEAD:data.txt) != second ]]; then
	echo "Git did not store the second file version" >&2
	exit 1
fi
