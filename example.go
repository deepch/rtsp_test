package main

import (
	"log"
	"os"
	"encoding/hex"

	"github.com/nareix/mp4"
)

func main() {
	RtspReader := RtspClientNew()

	quit := false

	sps := []byte{}
	pps := []byte{}
	fuBuffer := []byte{}
	syncCount := 0

	// rtp timestamp: 90 kHz clock rate
	// 1 sec = timestamp 90000
	timeScale := 90000

	type NALU struct {
		ts int
		data []byte
		sync bool
	}
	var lastNALU *NALU

	var mp4w *mp4.SimpleH264Writer
	outfile, _ := os.Create("out.mp4")

	endWriteNALU := func() {
		log.Println("finish write")
		if err := mp4w.Finish(); err != nil {
			panic(err)
		}
	}

	writeNALU := func(sync bool, ts int, payload []byte) {
		if mp4w == nil {
			mp4w = &mp4.SimpleH264Writer{
				SPS: sps,
				PPS: pps,
				TimeScale: timeScale,
				W: outfile,
			}
			log.Println("SPS:\n"+hex.Dump(sps), "\nPPS:\n"+hex.Dump(pps))
		}
		curNALU := &NALU{
			ts: ts,
			sync: sync,
			data: payload,
		}

		//log.Println(hex.Dump(payload))
		if lastNALU != nil {
			log.Println("write", len(payload))
			if err := mp4w.WriteNALU(lastNALU.sync, curNALU.ts - lastNALU.ts, lastNALU.data); err != nil {
				panic(err)
			}
		}
		lastNALU = curNALU
	}

	handleNALU := func(nalType byte, payload []byte, ts int64) {
		if nalType == 7 {
			sps = payload
		} else if nalType == 8 {
			pps = payload
		} else if nalType == 5 {
			// keyframe
			syncCount++
			if syncCount == 3 {
				quit = true
			}
			writeNALU(true, int(ts), payload)
		} else {
			// non-keyframe
			if syncCount > 0 {
				writeNALU(false, int(ts), payload)
			}
		}
	}

	if status, message := RtspReader.Client("rtsp://admin:123456@94.242.52.34:5543/mpeg4cif"); status {
		log.Println("connected")
		i := 0
		for {
			i++
			//read 100 frame and exit loop
			if quit {
				break
			}
			select {
			case data := <-RtspReader.outgoing:
				//log.Println("packet recive")
				if data[0] == 36 && data[1] == 0 {
					cc := data[4] & 0xF
					//rtp header
					rtphdr := 12 + cc*4

					//packet time
					ts := (int64(data[8]) << 24) + (int64(data[9]) << 16) + (int64(data[10]) << 8) + (int64(data[11]))

					//packet number
					packno := (int64(data[6]) << 8) + int64(data[7])
					if false {
						log.Println("packet num", packno)
					}

					nalType := data[4+rtphdr] & 0x1F

					if nalType >= 1 && nalType <= 23 {
						handleNALU(nalType, data[4+rtphdr:], ts)
					} else if nalType == 28 {
						fuBuffer = append(fuBuffer, data[4+rtphdr+2:]...)
						isEnd := data[4+rtphdr+1]&0x40 != 0
						nalType := data[4+rtphdr+1]&0x1F
						if isEnd {
							handleNALU(nalType, fuBuffer, ts)
							fuBuffer = []byte{}
						}
					}

				}

			case <-RtspReader.signals:
				log.Println("exit signal by class rtsp")
			}
		}
	} else {
		log.Println("error", message)
	}

	endWriteNALU()
	RtspReader.Close()
}
