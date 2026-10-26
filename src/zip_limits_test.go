package archive

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
)

func TestZIPDirectoryCountsActualHeadersBeforeAllocation(t *testing.T) {
	entries := make([][2]string, 501)
	for i := range entries {
		entries[i] = [2]string{fmt.Sprintf("%d.txt", i), "Quist"}
	}
	b := officeZip(t, entries...)
	end := len(b) - 22
	// A forged footer undercounts the real directory. Checking only the footer
	// would let archive/zip allocate every entry before rejecting the file.
	binary.LittleEndian.PutUint16(b[end+8:], 1)
	binary.LittleEndian.PutUint16(b[end+10:], 1)
	if err := CheckZIPDirectory(bytes.NewReader(b), int64(len(b)), 500, 4<<20); !errors.Is(err, ErrZIPLimit) {
		t.Fatalf("actual directory limit: %v", err)
	}
}

func TestZIPDirectoryBoundsAndZIP64(t *testing.T) {
	b := officeZip(t, [2]string{"Quist.txt", "A fictional picnic"})
	if err := CheckZIPDirectory(bytes.NewReader(b), int64(len(b)), 1, 4096); err != nil {
		t.Fatal(err)
	}
	if err := CheckZIPDirectory(bytes.NewReader(b), int64(len(b)), 0, 4096); !errors.Is(err, ErrZIPLimit) {
		t.Fatalf("count limit: %v", err)
	}
	if err := CheckZIPDirectory(bytes.NewReader(b), int64(len(b)), 1, 1); !errors.Is(err, ErrZIPLimit) {
		t.Fatalf("directory bytes limit: %v", err)
	}
	le := binary.LittleEndian
	end := len(b) - 22
	footer := append([]byte(nil), b[end:]...)
	var z64 [56]byte
	le.PutUint32(z64[:], 0x06064b50)
	le.PutUint64(z64[4:], 44)
	le.PutUint64(z64[24:], 1)
	le.PutUint64(z64[32:], 1)
	le.PutUint64(z64[40:], uint64(le.Uint32(footer[12:])))
	le.PutUint64(z64[48:], uint64(le.Uint32(footer[16:])))
	var locator [20]byte
	le.PutUint32(locator[:], 0x07064b50)
	le.PutUint64(locator[8:], uint64(end))
	le.PutUint32(locator[16:], 1)
	le.PutUint16(footer[8:], 65535)
	le.PutUint16(footer[10:], 65535)
	le.PutUint32(footer[12:], 0xffffffff)
	le.PutUint32(footer[16:], 0xffffffff)
	out := append(append(append(append([]byte(nil), b[:end]...), z64[:]...), locator[:]...), footer...)
	if err := CheckZIPDirectory(bytes.NewReader(out), int64(len(out)), 1, 4096); err != nil {
		t.Fatalf("small ZIP64: %v", err)
	}
	le.PutUint64(out[end+32:], ^uint64(0))
	if err := CheckZIPDirectory(bytes.NewReader(out), int64(len(out)), 1, 4096); !errors.Is(err, ErrZIPLimit) {
		t.Fatalf("forged ZIP64 count: %v", err)
	}
}
