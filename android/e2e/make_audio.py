"""Writes four 20-second test tones as WAV files into the given folder."""
import math, struct, sys, wave, os
out = sys.argv[1]
os.makedirs(out, exist_ok=True)
for n in range(4):
    with wave.open(os.path.join(out, f"{n}.wav"), "wb") as w:
        w.setnchannels(1); w.setsampwidth(2); w.setframerate(22050)
        f = 220 + 110 * n
        w.writeframes(b"".join(struct.pack("<h", int(8000 * math.sin(2 * math.pi * f * i / 22050))) for i in range(22050 * 20)))
