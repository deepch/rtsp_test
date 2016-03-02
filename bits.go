
package main

import (
	"io"
)

type BitReader struct {
	R io.Reader
	buf [1]byte
	left byte
}

func (self *BitReader) ReadBit() (res uint, err error) {
	if self.left == 0 {
		if _, err = self.R.Read(self.buf[:]); err != nil {
			return
		}
		self.left = 8
	}
	self.left--
	res = uint(self.buf[0]>>self.left)&1
	return
}

func (self *BitReader) ReadBits(n int) (res uint, err error) {
	for i := 0; i < n; i++ {
		var bit uint
		if bit, err = self.ReadBit(); err != nil {
			return
		}
		res |= bit<<uint(n-i-1)
	}
	return
}

type BitWriter struct {
	W io.Writer
	buf [1]byte
	written byte
}

func (self *BitWriter) WriteBits(n int, val uint) (err error) {
	for i := n-1; i >= 0; i-- {
		self.buf[0] <<= 1
		if val&(1<<uint(i)) != 0 {
			self.buf[0] |= 1
		}
		self.written++
		if self.written == 8 {
			if _, err = self.W.Write(self.buf[:]); err != nil {
				return
			}
			self.buf[0] = 0
			self.written = 0
		}
	}
	return
}

func (self *BitWriter) FlushBits() (err error) {
	if self.written > 0 {
		self.buf[0] <<= 8-self.written
		if _, err = self.W.Write(self.buf[:]); err != nil {
			return
		}
		self.written = 0
	}
	return
}

