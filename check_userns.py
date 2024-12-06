from typing import Any, Awaitable, Mapping, Optional, Sequence, Tuple, Type
from pathlib import Path
from collections.abc import Iterable, Iterator, Mapping, Sequence
import os
import pwd
import contextlib
import fcntl
from typing import IO, Union
import contextvars
import subprocess
import sys
import ctypes
import ctypes.util
import signal

SUBRANGE = 65536
CLONE_NEWNS = 0x00020000
CLONE_NEWUSER = 0x10000000

ARG_DEBUG = contextvars.ContextVar("debug", default=False)
_FILE = Union[None, int, IO[Any]]
PathString = Union[Path, str]
Popen = subprocess.Popen[str]

@contextlib.contextmanager
def flock(path: Path) -> Iterator[int]:
    fd = os.open(path, os.O_CLOEXEC|os.O_RDONLY)
    try:
        fcntl.fcntl(fd, fcntl.FD_CLOEXEC)
        fcntl.flock(fd, fcntl.LOCK_EX)
        yield fd
    finally:
        os.close(fd)

def read_subrange(path: Path) -> int:
    uid = str(os.getuid())
    try:
        user = pwd.getpwuid(os.getuid()).pw_name
    except KeyError:
        user = None

    for line in path.read_text().splitlines():
        name, start, count = line.split(":")

        if name == uid or name == user:
            break
    else:
        die(f"No mapping found for {user or uid} in {path}")

    if int(count) < SUBRANGE:
        die(f"subuid/subgid range length must be at least {SUBRANGE}, got {count} for {user or uid} from line '{line}'")

    return int(start)


class InvokingUser:
    @staticmethod
    def _uid_from_env() -> Optional[int]:
        uid = os.getenv("SUDO_UID") or os.getenv("PKEXEC_UID")
        return int(uid) if uid is not None else None

    @classmethod
    def uid(cls) -> int:
        return cls._uid_from_env() or os.getuid()

    @classmethod
    def uid_gid(cls) -> tuple[int, int]:
        if (uid := cls._uid_from_env()) is not None:
            gid = int(os.getenv("SUDO_GID", pwd.getpwuid(uid).pw_gid))
            return uid, gid
        return os.getuid(), os.getgid()

    @classmethod
    def name(cls) -> str:
        return pwd.getpwuid(cls.uid()).pw_name

    @classmethod
    def home(cls) -> Path:
        return Path(f"~{cls.name()}").expanduser()

    @classmethod
    def is_running_user(cls) -> bool:
        return cls.uid() == os.getuid()

def become_root() -> tuple[int, int]:
    """
    Set up a new user namespace mapping using /etc/subuid and /etc/subgid.

    The current user will be mapped to root and 65436 will be mapped to the UID/GID of the invoking user.
    The other IDs will be mapped through.

    The function returns the UID-GID pair of the invoking user in the namespace (65436, 65436).
    """
    if os.getuid() == 0:
        return InvokingUser.uid_gid()

    subuid = read_subrange(Path("/etc/subuid"))
    subgid = read_subrange(Path("/etc/subgid"))

    pid = os.getpid()


    # We map the private UID range configured in /etc/subuid and /etc/subgid into the container using
    # newuidmap and newgidmap. On top of that, we also make sure to map in the user running mkosi so that
    # we can run still chown stuff to that user or run stuff as that user which will make sure any
    # generated files are owned by that user. We don't map to the last user in the range as the last user
    # is sometimes used in tests as a default value and mapping to that user might break those tests.
    newuidmap = [
        "flock", "--exclusive", "--no-fork", "/etc/subuid", "newuidmap", pid,
        0, subuid, SUBRANGE - 100,
        SUBRANGE - 100, os.getuid(), 1,
        SUBRANGE - 100 + 1, subuid + SUBRANGE - 100 + 1, 99
    ]

    newgidmap = [
        "flock", "--exclusive", "--no-fork", "/etc/subuid", "newgidmap", pid,
        0, subgid, SUBRANGE - 100,
        SUBRANGE - 100, os.getgid(), 1,
        SUBRANGE - 100 + 1, subgid + SUBRANGE - 100 + 1, 99
    ]

    newuidmap = [str(x) for x in newuidmap]
    newgidmap = [str(x) for x in newgidmap]

    # newuidmap and newgidmap have to run from outside the user namespace to be able to assign a uid mapping
    # to the process in the user namespace. The mapping can only be assigned after the user namespace has
    # been unshared. To make this work, we first lock /etc/subuid, then spawn the newuidmap and newgidmap
    # processes, which we execute using flock so they don't execute before they can get a lock on /etc/subuid,
    # then we unshare the user namespace and finally we unlock /etc/subuid, which allows the newuidmap and
    # newgidmap processes to execute. we then wait for the processes to finish before continuing.
    with flock(Path("/etc/subuid")) as fd, spawn(newuidmap) as uidmap, spawn(newgidmap) as gidmap:
        unshare(CLONE_NEWUSER)
        fcntl.flock(fd, fcntl.LOCK_UN)
        uidmap.wait()
        gidmap.wait()

    # By default, we're root in the user namespace because if we were our current user by default, we
    # wouldn't be able to chown stuff to be owned by root while the reverse is possible.
    os.setresuid(0, 0, 0)
    os.setresgid(0, 0, 0)
    os.setgroups([0])

    return SUBRANGE - 100, SUBRANGE - 100

def spawn(
    cmdline: Sequence[PathString],
    stdin: _FILE = None,
    stdout: _FILE = None,
    stderr: _FILE = None,
    user: Optional[int] = None,
    group: Optional[int] = None,
) -> Popen:
    if ARG_DEBUG.get():
        logging.info(f"+ {shlex.join(os.fspath(s) for s in cmdline)}")

    if not stdout and not stderr:
        # Unless explicit redirection is done, print all subprocess
        # output on stderr, since we do so as well for mkosi's own
        # output.
        stdout = sys.stderr

    try:
        return subprocess.Popen(
            cmdline,
            stdin=stdin,
            stdout=stdout,
            stderr=stderr,
            text=True,
            user=user,
            group=group,
            preexec_fn=foreground,
        )
    except FileNotFoundError:
        die(f"{cmdline[0]} not found in PATH.")
    except subprocess.CalledProcessError as e:
        logging.error(f"\"{shlex.join(os.fspath(s) for s in cmdline)}\" returned non-zero exit code {e.returncode}.")
        raise e

def unshare(flags: int) -> None:
    libc_name = ctypes.util.find_library("c")
    if libc_name is None:
        die("Could not find libc")
    libc = ctypes.CDLL(libc_name, use_errno=True)

    if libc.unshare(ctypes.c_int(flags)) != 0:
        e = ctypes.get_errno()
        raise OSError(e, os.strerror(e))

def foreground(*, new_process_group: bool = True) -> None:
    """
    If we're connected to a terminal, put the process in a new process group and make that the foreground
    process group so that only this process receives SIGINT.
    """
    STDERR_FILENO = 2
    if os.isatty(STDERR_FILENO):
        if new_process_group:
            os.setpgrp()
        old = signal.signal(signal.SIGTTOU, signal.SIG_IGN)
        os.tcsetpgrp(STDERR_FILENO, os.getpgrp())
        signal.signal(signal.SIGTTOU, old)

print(f"uid: {os.getuid()}, gid: {os.getgid()}")
become_root()
print(f"uid: {os.getuid()}, gid: {os.getgid()}")
