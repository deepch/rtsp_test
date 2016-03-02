
package main

import (
	"testing"
	"bytes"
)

func TestBitWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	bw := &BitWriter{W: buf}
	bw.WriteBits(4, 0xa)
	bw.WriteBits(4, 0xb)
	bw.WriteBits(2, 0x1)
	bw.WriteBits(2, 0x1)
	bw.FlushBits()
	b := buf.Bytes()
	if b[0] != 0xab || b[1] != 0x50 {
		t.Fail()
	}
}

