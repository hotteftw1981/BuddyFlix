#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit("usage: fix_qpkg_tail.py <package.qpkg>")

path = Path(sys.argv[1])
data = bytearray(path.read_bytes())

if len(data) < 100 or data[-10:] != b"QNAPQPKG  ":
    raise SystemExit("unexpected QPKG footer")

# QTS expects the legacy 10-byte QNAP package check field used by the
# known-good TS-431P2 package. QDK can leave this field blank on Linux builds.
check = str(len(data) * 3589 + 1000000000)[:10].encode("ascii")
data[-60:-50] = check
path.write_bytes(data)

print(f"patched {path.name}: qnap_tail_check={check.decode()}")
