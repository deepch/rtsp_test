
package main

import (
	"io"
	"bytes"
)

type Mpeg4AudioConfig struct {
	ObjectType uint
	ChannelConf uint
	SampleRateIndex uint
	SizeLength int
	IndexLength int
}

func WriteBitsADTSFrameHeader(w io.Writer, config Mpeg4AudioConfig, size uint) {
	bw := &BitWriter{W: w}
	fullFrameSize := 7 + size

	bw.WriteBits(12, 0xfff)   /* syncword */
	bw.WriteBits(1, 0)        /* ID */
	bw.WriteBits(2, 0)        /* layer */
	bw.WriteBits(1, 1)        /* protection_absent */

	bw.WriteBits(2, config.ObjectType-1) /* profile_objecttype */
	bw.WriteBits(4, config.SampleRateIndex)
	bw.WriteBits(1, 0)        /* private_bit */
	bw.WriteBits(3, config.ChannelConf) /* channel_configuration */
	bw.WriteBits(1, 0)        /* original_copy */
	bw.WriteBits(1, 0)        /* home */

	/* adts_variable_header */
	bw.WriteBits(1, 0)        /* copyright_identification_bit */
	bw.WriteBits(1, 0)        /* copyright_identification_start */
	bw.WriteBits(13, fullFrameSize) /* aac_frame_length */
	bw.WriteBits(11, 0x7ff)   /* adts_buffer_fullness */
	bw.WriteBits(2, 0)        /* number_of_raw_data_blocks_in_frame */

	bw.FlushBits()
}

func ParseAUFrame(frame []byte, config Mpeg4AudioConfig) (data []byte) {
	br := bytes.NewReader(frame)
	r := &BitReader{R: br}

	headersLength, _ := r.ReadBits(16)
	headersLengthBytes := int(headersLength+7)/8
	headerSize := config.IndexLength + config.SizeLength
	nHeaders := int(headersLength) / headerSize

	if nHeaders > 0 {
		size, _ := r.ReadBits(config.SizeLength)
		skip := 2+headersLengthBytes
		if len(frame) < skip+int(size) {
			return
		}
		data = frame[skip:skip+int(size)]
		return
	}

	return
}

func ParseAudioSpecificConfig(r io.Reader, config *Mpeg4AudioConfig) (err error) {
	br := &BitReader{R: r}

	if config.ObjectType, err = br.ReadBits(5); err != nil {
		return
	}
	if config.ObjectType == 31 {
		if config.ObjectType, err = br.ReadBits(6); err != nil {
			return
		}
		config.ObjectType += 32
	}

	if config.SampleRateIndex, err = br.ReadBits(4); err != nil {
		if config.SampleRateIndex == 0xf {
			if config.SampleRateIndex, err = br.ReadBits(24); err != nil {
				return
			}
		}
		return
	}

	if config.ChannelConf, err = br.ReadBits(4); err != nil {
		return
	}

	return
}

