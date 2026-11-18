"""Checks that a Windows .exe carries the Gundil Music icon and file details.

    python scripts/check-exe-icon.py gundil-music.exe
"""
import struct
import sys

b = open(sys.argv[1], "rb").read()
pe = struct.unpack("<I", b[0x3C:0x40])[0]
n = struct.unpack("<H", b[pe + 6:pe + 8])[0]
opt = struct.unpack("<H", b[pe + 20:pe + 22])[0]
magic = struct.unpack("<H", b[pe + 24:pe + 26])[0]
rva, size = struct.unpack("<II", b[pe + 24 + (112 if magic == 0x20B else 96) + 16:][:8])
if not rva or not size:
    sys.exit("no resources in " + sys.argv[1])
ptr = None
for i in range(n):
    o = pe + 24 + opt + i * 40
    va, raw = struct.unpack("<I", b[o + 12:o + 16])[0], struct.unpack("<I", b[o + 20:o + 24])[0]
    vsize = struct.unpack("<I", b[o + 8:o + 12])[0]
    if va <= rva < va + vsize:
        ptr = raw + (rva - va)
if ptr is None:
    sys.exit("resource table not found")
named, ids = struct.unpack("<HH", b[ptr + 12:ptr + 16])
types = {struct.unpack("<I", b[ptr + 16 + 8 * i:ptr + 20 + 8 * i])[0] for i in range(named + ids)}
missing = [name for t, name in ((3, "icon"), (14, "icon group"), (16, "version info")) if t not in types]
if missing or "Gundil Music".encode("utf-16-le") not in b:
    sys.exit("missing: " + ", ".join(missing or ["the Gundil Music file details"]))
print("icon and file details present")
