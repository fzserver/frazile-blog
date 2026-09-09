---
title: Recovering a zip archive with a broken central directory
slug: recover-zip-broken-central-directory
summary: A 20 GB download where the first six entries extracted and the other ten came out as zero-byte files. The data was fine; only the index was wrong. A short Python script that trusts the file headers instead of the index got everything back with matching checksums.
tags: homelab, python, debugging, linux, archives
days_ago: 0
---
A large zip finished downloading overnight. `unzip` listed sixteen entries, extracted six of them and produced zero-byte files for the rest. `7z x` was more honest: "Headers Error", then the same result. The download had taken most of a night on a slow mirror, so before starting it again I wanted to know what exactly was broken. It turned out to be the least important part of the file.

![Vibrant green numbers on a computer screen, showcasing binary code and data streams.](/media/2026/09/d5b5c56552d08162.jpg)
*Photo by [Tibe De Kort](https://www.pexels.com/@tibe-de-kort-115101682) on [Pexels](https://www.pexels.com)*

## How a zip is laid out

A zip is a sequence of entries, each one a local header (signature `PK\x03\x04`, filename, sizes, CRC) followed by the compressed bytes. At the very end of the file sits the **central directory**: a second copy of every entry's metadata plus the byte offset of its local header. Extractors read the central directory first and jump to each offset from there. They never scan the file.

That design is why a zip can be corrupt in two very different ways. If the compressed data is damaged, nothing can help. If only the central directory is wrong, every byte you need is still there; the map is just pointing at the wrong places. The symptom of the second case is exactly what I had: entries at the start work, entries after a certain point do not, and the failures are clean (empty output, checksum complaints) rather than garbage.

A quick check confirmed it. Searching the file for local header signatures found sixteen of them, and the filenames next to them were the right filenames. The central directory's offsets for entries seven onward were simply wrong, probably the result of whatever tool built the archive on the other end streaming it and writing offsets from a different base.

## Trust the headers, verify with the index

The script does the opposite of an extractor. It reads the central directory only for what it is good for, filenames, expected sizes and CRC32s, and then finds the data itself.

```python
import mmap, struct, zlib, zipfile

def local_headers(mm, wanted):
    pos = 0
    while True:
        pos = mm.find(b"PK\x03\x04", pos)
        if pos < 0:
            return
        flags, method, _, _, crc, csize, usize, nlen, xlen = struct.unpack("<HHHHIIIHH", mm[pos+6:pos+30])
        name = mm[pos+30:pos+30+nlen].decode("utf-8", "replace")
        if name in wanted:
            yield name, method, pos + 30 + nlen + xlen
        pos += 4
```

Scanning for `PK\x03\x04` alone is not enough: four bytes will occur by accident inside compressed data every few gigabytes. Filtering hits by whether the filename that follows matches one from the central directory removes the false positives. The whole file is memory-mapped, so the scan is a single pass over the disk and needs almost no RAM.

For each real header the script decompresses from that offset. Two details made the difference between "works on the test archive" and "works on the real one":

- **Streaming deflate until it terminates.** Archives written in streaming mode set a flag that says the sizes in the local header are zero and the real ones follow the data. So the local header cannot tell you how many bytes to read. `zlib.decompressobj()` handles this: feed it chunks and stop when it reports the stream has ended (`unused_data` becomes non-empty).
- **Stored entries need the size from somewhere.** For method 0 there is nothing to terminate on, so the size comes from the central directory, which is still right about sizes even when it is wrong about offsets.

Every output is then checked against the central directory's CRC32 and uncompressed size. If either mismatches, the file is deleted and reported. Nothing is trusted until it has been verified twice.

![High-angle view of woman coding on a laptop, with a Python book nearby. Ideal for programming and tech content.](/media/2026/09/65645ec076ed97b0.jpg)
*Photo by [Christina Morillo](https://www.pexels.com/@divinetechygirl) on [Pexels](https://www.pexels.com)*

## Running it

```sh
python3 recover_zip.py archive.zip out/           # every entry
python3 recover_zip.py archive.zip out/ ep07.mkv  # just one
```

It skips outputs that already exist with the correct size, so it can be re-run after a crash or interrupted halfway. On the 20 GB archive it ran at disk speed and finished with sixteen files, all CRCs matching. The re-download never happened.

## When this does not help

If the local headers are missing or the compressed data itself is damaged, this approach fails at the CRC step, which is the right way to fail. If the archive is one of the multi-part variants with data spanning files, the scan needs to run across the concatenation. And if a filename in the central directory does not match the one in the local header (rare, but tools exist that rewrite one and not the other), the filter drops it; matching on CRC instead of name would cover that case.

The general lesson survived the specific script: before redoing expensive work, find out which part is broken. The index and the data are different things, and the tools that gave up were only ever looking at the index.

---
*Cover photo by [Ivo Brasil](https://www.pexels.com/@ivo-brasil-335441) on [Pexels](https://www.pexels.com).*
