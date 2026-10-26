package archive

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"io"
)

// ErrZIPLimit means that a ZIP directory exceeds an explicitly supplied bound.
var ErrZIPLimit = errors.New("ZIP directory exceeds the processing limit")

// CheckZIPDirectory bounds both declared and actual entry counts before
// archive/zip allocates its per-entry structures. It reads at most a 64 KiB
// footer and one fixed-size header at a time. It accepts ordinary single-disk
// ZIP/ZIP64 archives, without executable prefixes or trailing data.
func CheckZIPDirectory(r io.ReaderAt, size int64, maxEntries, maxDirectoryBytes uint64) error {
	if size < 22 {
		return zip.ErrFormat
	}
	tail := make([]byte, min(size, 22+65535))
	start := size - int64(len(tail))
	if _, err := r.ReadAt(tail, start); err != nil {
		return err
	}
	le := binary.LittleEndian
	i := len(tail) - 22
	for ; i >= 0; i-- {
		if le.Uint32(tail[i:]) == 0x06054b50 && i+22+int(le.Uint16(tail[i+20:])) == len(tail) {
			break
		}
	}
	if i < 0 {
		return zip.ErrFormat
	}
	e := tail[i:]
	if le.Uint16(e[4:]) != 0 || le.Uint16(e[6:]) != 0 {
		return zip.ErrFormat
	}
	count, diskCount := uint64(le.Uint16(e[10:])), uint64(le.Uint16(e[8:]))
	dirSize, offset := uint64(le.Uint32(e[12:])), uint64(le.Uint32(e[16:]))
	end := uint64(start + int64(i))
	if count == 65535 || diskCount == 65535 || dirSize == 0xffffffff || offset == 0xffffffff {
		if end < 20 {
			return zip.ErrFormat
		}
		var locator [20]byte
		if _, err := r.ReadAt(locator[:], int64(end)-20); err != nil {
			return err
		}
		if le.Uint32(locator[:]) != 0x07064b50 || le.Uint32(locator[4:]) != 0 || le.Uint32(locator[16:]) != 1 {
			return zip.ErrFormat
		}
		at := le.Uint64(locator[8:])
		if at > end-20 || end-20-at < 56 {
			return zip.ErrFormat
		}
		var record [56]byte
		if _, err := r.ReadAt(record[:], int64(at)); err != nil {
			return err
		}
		length := le.Uint64(record[4:])
		if le.Uint32(record[:]) != 0x06064b50 || length < 44 || length != end-20-at-12 || le.Uint32(record[16:]) != 0 || le.Uint32(record[20:]) != 0 {
			return zip.ErrFormat
		}
		diskCount, count = le.Uint64(record[24:]), le.Uint64(record[32:])
		dirSize, offset = le.Uint64(record[40:]), le.Uint64(record[48:])
		end = at
	}
	if count > maxEntries || dirSize > maxDirectoryBytes {
		return ErrZIPLimit
	}
	if count != diskCount || offset > end || dirSize != end-offset {
		return zip.ErrFormat
	}
	var header [46]byte
	var actual uint64
	for pos := offset; pos < end; {
		actual++
		if actual > maxEntries {
			return ErrZIPLimit
		}
		if end-pos < 46 {
			return zip.ErrFormat
		}
		if _, err := r.ReadAt(header[:], int64(pos)); err != nil {
			return err
		}
		if le.Uint32(header[:]) != 0x02014b50 || le.Uint16(header[34:]) != 0 {
			return zip.ErrFormat
		}
		n := uint64(46) + uint64(le.Uint16(header[28:])) + uint64(le.Uint16(header[30:])) + uint64(le.Uint16(header[32:]))
		if n > end-pos {
			return zip.ErrFormat
		}
		pos += n
	}
	if actual != count {
		return zip.ErrFormat
	}
	return nil
}
