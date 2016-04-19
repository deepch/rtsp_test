package main

import (
	"log"
	"os"
	"encoding/gob"
	"encoding/hex"
	"flag"

	"github.com/nareix/mp4"
	mpegts "github.com/nareix/ts"
)

var (
	VideoWidth int
	VideoHeight int
)

type GobAllSamples struct {
	TimeScale int
	SPS []byte
	PPS []byte
	Samples []GobSample
}

type GobSample struct {
	Type int
	Ts int
	Data []byte
	Sync bool
}

func main() {
	var saveGob bool
	var url string
	var maxgop int

	// with aac rtsp://admin:123456@80.254.21.110:554/mpeg4cif
	// with aac rtsp://admin:123456@95.31.251.50:5050/mpeg4cif
	// 1808p rtsp://admin:123456@171.25.235.18/mpeg4
	// 640x360 rtsp://admin:123456@94.242.52.34:5543/mpeg4cif

	flag.BoolVar(&saveGob, "s", false, "save to gob file")
	flag.IntVar(&maxgop, "g", 4, "max gop recording")
	flag.StringVar(&url, "url", "rtsp://admin:123456@94.242.52.34:5543/mpeg4cif", "")
	flag.Parse()

	RtspReader := RtspClientNew()

	quit := false

	sps := []byte{}
	pps := []byte{}
	fuBuffer := []byte{}
	syncCount := 0

	var mp4mux *mp4.Muxer
	var tsmux *mpegts.Muxer
	var mp4H264Track *mp4.Track
	var mp4AACTrack *mp4.Track
	var tsH264Track *mpegts.Track
	var tsAACTrack *mpegts.Track

	var allSamples *GobAllSamples

	outfileMp4, _ := os.Create("out.mp4")
	outfileTs, _ := os.Create("out.ts")

	endWriteNALU := func() {
		log.Println("finish write")
		if mp4mux != nil {
			if err := mp4mux.WriteTrailer(); err != nil {
				panic(err)
			}
		}
		outfileTs.Close()

		if saveGob {
			file, _ := os.Create("out.gob")
			enc := gob.NewEncoder(file)
			enc.Encode(allSamples)
			file.Close()
		}
	}

	writeNALU := func(sync bool, ts int, payload []byte) {
		if saveGob && allSamples == nil {
			allSamples = &GobAllSamples{
				SPS: sps,
				PPS: pps,
				TimeScale: 90000,
			}
		}

		if false {
			log.Println("write", sync, len(payload))
		}

		if mp4mux == nil {
			mp4mux = &mp4.Muxer{
				W: outfileMp4,
			}
			mp4H264Track = mp4mux.AddH264Track()
			mp4H264Track.SetH264PPSAndSPS(pps, sps)
			mp4H264Track.SetTimeScale(90000)

			if audioConfig != nil {
				mp4AACTrack = mp4mux.AddAACTrack()
				mp4AACTrack.SetTimeScale(16000)
			}

			if err := mp4mux.WriteHeader(); err != nil {
				panic(err)
			}
		}

		if tsmux == nil {
			tsmux = &mpegts.Muxer{
				W: outfileTs,
			}
			tsH264Track = tsmux.AddH264Track()
			tsH264Track.SPS = sps
			tsH264Track.PPS = pps
			tsH264Track.SetTimeScale(90000)

			if audioConfig != nil {
				tsAACTrack = tsmux.AddAACTrack()
				tsAACTrack.SetTimeScale(16000)
			}

			if err := tsmux.WriteHeader(); err != nil {
				panic(err)
			}
		}

		if true {
			log.Println("writeH264", ts, "\n"+hex.Dump(payload[:10]))
		}
		if err := tsH264Track.WriteSample(int64(ts), int64(ts), sync, payload); err != nil {
			panic(err)
		}
		if err := mp4H264Track.WriteSample(int64(ts), int64(ts), sync, payload); err != nil {
			panic(err)
		}

		if saveGob {
			allSamples.Samples = append(allSamples.Samples, GobSample{
				Sync: sync,
				Ts: ts,
				Data: payload,
			})
		}
	}

	var firstAACTs, firstH264Ts int64

	handleNALU := func(nalType byte, payload []byte, ts int64) {
		if firstH264Ts == 0 {
			firstH264Ts = ts
		}
		ts -= firstH264Ts

		if nalType == 7 {
			if len(sps) == 0 {
				sps = payload
			}
		} else if nalType == 8 {
			if len(pps) == 0 {
				pps = payload
			}
		} else if nalType == 5 {
			// keyframe
			syncCount++
			if syncCount == maxgop {
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

	handleAACFrame := func(frame []byte, ts int64) {
	

		if true {
			log.Println("writeAAC", ts, len(frame))
		}

		if tsAACTrack != nil {
			if firstAACTs == 0 {
				firstAACTs = ts
			}
			ts -= firstAACTs
			if err := tsAACTrack.WriteSample(ts, ts, true, frame); err != nil {
				panic(err)
			}
		}
		if mp4AACTrack != nil {
			if err := mp4AACTrack.WriteSample(ts, ts, true, frame); err != nil {
				panic(err)
			}
		}
	}

	if status, message := RtspReader.Client(url); status {
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

				if true {
					log.Printf("packet type=%d len=%d\n", data[1], len(data))
				}

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
						isStart := data[4+rtphdr+1]&0x80 != 0
						isEnd := data[4+rtphdr+1]&0x40 != 0
						nalType := data[4+rtphdr+1]&0x1F
						//nri := (data[4+rtphdr+1]&0x60)>>5
						nal := data[4+rtphdr]&0xE0 | data[4+rtphdr+1]&0x1F
						if isStart {
							fuBuffer = []byte{0}
						}
						fuBuffer = append(fuBuffer, data[4+rtphdr+2:]...)
						if isEnd {
							fuBuffer[0] = nal
							handleNALU(nalType, fuBuffer, ts)
						}
					}

				} else if data[0] == 36 && data[1] == 2 {
					// audio
					cc := data[4] & 0xF
					rtphdr := 12 + cc*4
					//packet time
					ts := (int64(data[8]) << 24) + (int64(data[9]) << 16) + (int64(data[10]) << 8) + (int64(data[11]))

					payload := data[4+rtphdr:]
					frame := ParseAUFrame(payload, *audioConfig)
					handleAACFrame(frame, ts)
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
