# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

import sys
import shlex
import subprocess
import atexit

if sys.stdout.isatty():
    def bold_print(fmt, *args, **kwargs):
        print('\033[1m' + fmt + '\033[0m', *args, **kwargs)
else:
    def bold_print(*args, **kwargs):
        print(*args, **kwargs)

# all running pids
processes = []

def process_status(p, fmt, *args, **kwargs):
    print(repr(shlex.join(p.args)) + ': ' + fmt, *args, **kwargs)

def terminate_all_processes():
    for process in processes:
        if process.poll() is None: # not done yet
            process_status(process, 'terminating')
            process.terminate()
            try:
                process.wait(timeout=10) # wait at most 10 seconds
            except subprocess.TimeoutExpired:
                print('Process not terminating after 10 seconds, killing it')
                process.kill()

atexit.register(terminate_all_processes)

def run_cmd(*args, **kwargs):
    p = subprocess.Popen(*args, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, **kwargs)
    process_status(p, 'start')
    processes.append(p)
    return p

def wait_cmd(p, timeout=None):
    try:
        output, _ = p.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        return False
    process_status(p, f'finished ({p.returncode})')
    if len(output) > 0:
        process_status(p, 'output:')
        sys.stdout.flush()
        sys.stdout.buffer.write(output)
        sys.stdout.flush()
    else:
        process_status(p, 'no output')
    if p.returncode != 0:
        sys.exit(p.returncode)
    return True

def wait_cmds(ps):
    done = [False]*len(ps)
    while not all(done):
        for i, p in enumerate(ps):
            if done[i]:
                continue
            done[i] = wait_cmd(p, timeout=0.1)

def run_cmd_unbuffered(*args, **kwargs):
    p = subprocess.Popen(*args, **kwargs)
    processes.append(p)
    p.communicate()
    if p.returncode != 0:
        sys.exit(p.returncode)
    return p
