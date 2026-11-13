// Copyright (C) 2026  Davi Silva Valério Diniz <davi.diniz101@gmail.com>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <https://www.gnu.org/licenses/>.
package main

import (
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	mp4 "github.com/abema/go-mp4"
	"github.com/jfreymuth/oggvorbis"
	"github.com/mewkiz/flac"
)

func ReadDuration(path string) (time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return ReadDurationFile(f)
}

func ReadDurationFile(f *os.File) (time.Duration, error) {
	switch strings.ToLower(filepath.Ext(f.Name())) {
	case ".wav":
		return readDurationWAV(f)
	case ".mp3":
		return readDurationMP3(f)
	case ".mp4", ".m4a":
		return readDurationMP4(f)
	case ".webm":
		return readDurationWebM(f)
	case ".vgz":
		return readDurationVGZ(f)
	case ".flac":
		return readDurationFLAC(f)
	case ".ogg":
		return readDurationOGG(f)
	case ".opus":
		return readDurationOpus(f)
	case ".spc":
		return readDurationSPC(f)
	default:
		return 0, fmt.Errorf("unsupported extension: %s", filepath.Ext(f.Name()))
	}
}

func toDuration(seconds float64) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func readDurationWAV(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	var riff [12]byte
	if _, err := io.ReadFull(f, riff[:]); err != nil {
		return 0, err
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return 0, errors.New("wav: not a RIFF/WAVE file")
	}

	var sampleRate, channels, bitsPerSample, dataSize uint32
	for {
		var chunkID [4]byte
		var chunkSize uint32
		if _, err := io.ReadFull(f, chunkID[:]); err != nil {
			break
		}
		if err := binary.Read(f, binary.LittleEndian, &chunkSize); err != nil {
			break
		}

		switch string(chunkID[:]) {
		case "fmt ":
			var fmtCode uint16
			var ch uint16
			var bps uint16
			if err := binary.Read(f, binary.LittleEndian, &fmtCode); err != nil {
				return 0, err
			}
			if err := binary.Read(f, binary.LittleEndian, &ch); err != nil {
				return 0, err
			}
			if err := binary.Read(f, binary.LittleEndian, &sampleRate); err != nil {
				return 0, err
			}
			if _, err := f.Seek(6, io.SeekCurrent); err != nil {
				return 0, err
			}
			if err := binary.Read(f, binary.LittleEndian, &bps); err != nil {
				return 0, err
			}
			channels = uint32(ch)
			bitsPerSample = uint32(bps)
			if remain := int64(chunkSize) - 16; remain > 0 {
				if _, err := f.Seek(remain, io.SeekCurrent); err != nil {
					return 0, err
				}
			}
		case "data":
			dataSize = chunkSize
			goto done
		default:
			if _, err := f.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return 0, err
			}
		}
	}

done:
	if sampleRate == 0 || channels == 0 || bitsPerSample == 0 || dataSize == 0 {
		return 0, errors.New("wav: missing fmt or data chunk")
	}
	bytesPerSec := sampleRate * channels * (bitsPerSample / 8)
	if bytesPerSec == 0 {
		return 0, errors.New("wav: invalid format")
	}
	return toDuration(float64(dataSize) / float64(bytesPerSec)), nil
}

var mpegSampleRates = [4][4]int{
	{11025, 12000, 8000, 0},
	{0, 0, 0, 0},
	{22050, 24000, 16000, 0},
	{44100, 48000, 32000, 0},
}

var mpegBitrates = [2][3][16]int{
	{
		{0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256, 0},
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
	},
	{
		{0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448, 0},
		{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0},
		{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
	},
}

func readDurationMP3(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	id3Offset, err := skipID3v2(f)
	if err != nil {
		return 0, err
	}

	buf := make([]byte, 4096)
	n, err := io.ReadFull(f, buf)
	if err != nil && n == 0 {
		return 0, err
	}
	buf = buf[:n]

	sync := -1
	for i := 0; i+3 < len(buf); i++ {
		if buf[i] == 0xFF && (buf[i+1]&0xE0) == 0xE0 {
			sync = i
			break
		}
	}
	if sync < 0 {
		return 0, errors.New("mp3: no sync word found")
	}

	h := binary.BigEndian.Uint32(buf[sync : sync+4])
	verBits := int((h >> 19) & 0x3)
	layerBits := int((h >> 17) & 0x3)
	brIdx := int((h >> 12) & 0xF)
	srIdx := int((h >> 10) & 0x3)
	chMode := int((h >> 6) & 0x3)

	if layerBits == 0 || verBits == 1 {
		return 0, errors.New("mp3: reserved version/layer")
	}
	layer := 4 - layerBits
	sampleRate := mpegSampleRates[verBits][srIdx]
	if sampleRate == 0 {
		return 0, errors.New("mp3: unknown sample rate")
	}

	samplesPerFrame := 1152
	if layer == 1 {
		samplesPerFrame = 384
	} else if layer == 3 && verBits != 3 {
		samplesPerFrame = 576
	}

	sideInfoLen := 17
	if verBits == 3 && chMode != 3 {
		sideInfoLen = 32
	}
	if verBits != 3 && chMode == 3 {
		sideInfoLen = 9
	}

	xing := sync + 4 + sideInfoLen
	if xing+12 <= len(buf) {
		mark := string(buf[xing : xing+4])
		if mark == "Xing" || mark == "Info" {
			flags := binary.BigEndian.Uint32(buf[xing+4 : xing+8])
			if flags&0x1 != 0 {
				frames := binary.BigEndian.Uint32(buf[xing+8 : xing+12])
				return toDuration(float64(frames) * float64(samplesPerFrame) / float64(sampleRate)), nil
			}
		}
	}

	vbri := sync + 36
	if vbri+18 <= len(buf) && string(buf[vbri:vbri+4]) == "VBRI" {
		frames := binary.BigEndian.Uint32(buf[vbri+14 : vbri+18])
		return toDuration(float64(frames) * float64(samplesPerFrame) / float64(sampleRate)), nil
	}

	verIdx := 0
	if verBits == 3 {
		verIdx = 1
	}
	bitrate := mpegBitrates[verIdx][layer-1][brIdx]
	if bitrate == 0 {
		return 0, errors.New("mp3: free/unknown bitrate")
	}
	audioBytes := info.Size() - id3Offset
	return toDuration(float64(audioBytes) * 8 / float64(bitrate*1000)), nil
}

func readDurationMP4(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	boxes, err := mp4.ExtractBoxWithPayload(f, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeMvhd()})
	if err != nil {
		return 0, fmt.Errorf("mp4: %w", err)
	}
	if len(boxes) == 0 {
		return 0, errors.New("mp4: no mvhd box found")
	}
	mvhd, ok := boxes[0].Payload.(*mp4.Mvhd)
	if !ok || mvhd.Timescale == 0 {
		return 0, errors.New("mp4: invalid mvhd")
	}
	return toDuration(float64(mvhd.GetDuration()) / float64(mvhd.Timescale)), nil
}

func readEBMLVINT(r io.Reader) (uint64, bool, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, false, err
	}
	width := 1
	mask := byte(0x80)
	for mask > 0 && (b[0]&mask) == 0 {
		mask >>= 1
		width++
	}
	if mask == 0 {
		return 0, false, errors.New("ebml: invalid VINT")
	}

	val := uint64(b[0] &^ mask)
	rest := make([]byte, width-1)
	if _, err := io.ReadFull(r, rest); err != nil {
		return 0, false, err
	}
	for _, x := range rest {
		val = (val << 8) | uint64(x)
	}
	allOnes := uint64((1 << (7 * width)) - 1)
	return val, val == allOnes, nil
}

func readEBMLID(r io.Reader) (uint32, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	mask := byte(0x80)
	width := 1
	for (b[0] & mask) == 0 {
		mask >>= 1
		width++
		if mask == 0 {
			return 0, errors.New("ebml: invalid ID")
		}
	}
	id := uint32(b[0])
	rest := make([]byte, width-1)
	if _, err := io.ReadFull(r, rest); err != nil {
		return 0, err
	}
	for _, x := range rest {
		id = (id << 8) | uint32(x)
	}
	return id, nil
}

const (
	ebmlIDHeader    = 0x1A45DFA3
	ebmlIDSegment   = 0x18538067
	ebmlIDInfo      = 0x1549A966
	ebmlIDCluster   = 0x1F43B675
	ebmlIDDuration  = 0x4489
	ebmlIDTimeScale = 0x2AD7B1
)

func readDurationWebM(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	id, err := readEBMLID(f)
	if err != nil {
		return 0, err
	}
	if id != ebmlIDHeader {
		return 0, errors.New("webm: not an EBML file")
	}
	headerSize, unknown, err := readEBMLVINT(f)
	if err != nil {
		return 0, err
	}
	if !unknown {
		if _, err := f.Seek(int64(headerSize), io.SeekCurrent); err != nil {
			return 0, err
		}
	}

	id, err = readEBMLID(f)
	if err != nil {
		return 0, err
	}
	if id != ebmlIDSegment {
		return 0, errors.New("webm: no Segment element")
	}
	if _, _, err := readEBMLVINT(f); err != nil {
		return 0, err
	}

	timeScale := uint64(1_000_000)
	dur := -1.0

	for {
		id, err = readEBMLID(f)
		if err != nil {
			break
		}
		size, unknownSize, err := readEBMLVINT(f)
		if err != nil {
			break
		}

		switch id {
		case ebmlIDInfo:
			start, _ := f.Seek(0, io.SeekCurrent)
			for {
				if !unknownSize {
					cur, _ := f.Seek(0, io.SeekCurrent)
					if cur-start >= int64(size) {
						break
					}
				}
				subID, err := readEBMLID(f)
				if err != nil {
					break
				}
				subSize, _, err := readEBMLVINT(f)
				if err != nil {
					break
				}

				switch subID {
				case ebmlIDDuration:
					if subSize == 4 {
						var raw [4]byte
						if _, err := io.ReadFull(f, raw[:]); err != nil {
							return 0, err
						}
						dur = float64(math.Float32frombits(binary.BigEndian.Uint32(raw[:])))
					} else if subSize == 8 {
						var raw [8]byte
						if _, err := io.ReadFull(f, raw[:]); err != nil {
							return 0, err
						}
						dur = math.Float64frombits(binary.BigEndian.Uint64(raw[:]))
					} else {
						if _, err := f.Seek(int64(subSize), io.SeekCurrent); err != nil {
							return 0, err
						}
					}
				case ebmlIDTimeScale:
					if subSize > 8 {
						if _, err := f.Seek(int64(subSize), io.SeekCurrent); err != nil {
							return 0, err
						}
						break
					}
					var raw [8]byte
					if _, err := io.ReadFull(f, raw[8-subSize:]); err != nil {
						return 0, err
					}
					timeScale = binary.BigEndian.Uint64(raw[:])
				default:
					if _, err := f.Seek(int64(subSize), io.SeekCurrent); err != nil {
						return 0, err
					}
				}
				if dur >= 0 {
					return toDuration(dur * float64(timeScale) / float64(time.Second)), nil
				}
			}
		case ebmlIDCluster:
			return 0, errors.New("webm: no duration in Info element")
		default:
			if !unknownSize {
				if _, err := f.Seek(int64(size), io.SeekCurrent); err != nil {
					return 0, err
				}
			}
		}
	}

	return 0, errors.New("webm: duration not found")
}

func readDurationVGZ(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, fmt.Errorf("vgz: %w", err)
	}
	defer gz.Close()

	var header [36]byte
	if _, err := io.ReadFull(gz, header[:]); err != nil {
		return 0, fmt.Errorf("vgz: %w", err)
	}
	if string(header[0:4]) != "Vgm " {
		return 0, errors.New("vgz: not a VGM file")
	}
	samples := binary.LittleEndian.Uint32(header[0x18:0x1C])
	if samples == 0 {
		return 0, errors.New("vgz: zero total samples")
	}
	return toDuration(float64(samples) / 44100.0), nil
}

func readDurationFLAC(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	stream, err := flac.Parse(f)
	if err != nil {
		return 0, fmt.Errorf("flac: %w", err)
	}
	defer stream.Close()
	if stream.Info == nil || stream.Info.SampleRate == 0 || stream.Info.NSamples == 0 {
		return 0, errors.New("flac: invalid STREAMINFO")
	}
	return toDuration(float64(stream.Info.NSamples) / float64(stream.Info.SampleRate)), nil
}

func readDurationOGG(f *os.File) (time.Duration, error) {
	codec, err := detectOGGCodec(f)
	if err != nil {
		return 0, err
	}
	if codec == "vorbis" {
		return readDurationOGGVorbis(f)
	}
	return readDurationOpus(f)
}

func skipID3v2(f *os.File) (int64, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	var h [10]byte
	if _, err := io.ReadFull(f, h[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			_, seekErr := f.Seek(0, io.SeekStart)
			return 0, seekErr
		}
		return 0, err
	}
	if string(h[0:3]) != "ID3" {
		_, err := f.Seek(0, io.SeekStart)
		return 0, err
	}
	size := int64(h[6])<<21 | int64(h[7])<<14 | int64(h[8])<<7 | int64(h[9])
	offset := int64(10) + size
	_, err := f.Seek(offset, io.SeekStart)
	return offset, err
}

func detectOGGCodec(f *os.File) (string, error) {
	if _, err := skipID3v2(f); err != nil {
		return "", err
	}

	var hdr [27]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return "", err
	}
	if string(hdr[0:4]) != "OggS" {
		return "", errors.New("ogg: not an Ogg file")
	}
	segCount := int(hdr[26])
	segTable := make([]byte, segCount)
	if _, err := io.ReadFull(f, segTable); err != nil {
		return "", err
	}
	payloadLen := 0
	for _, s := range segTable {
		payloadLen += int(s)
	}
	if payloadLen > 8 {
		payloadLen = 8
	}
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(f, payload); err != nil {
		return "", err
	}
	if len(payload) >= 8 && string(payload[:8]) == "OpusHead" {
		return "opus", nil
	}
	if len(payload) >= 7 && payload[0] == 0x01 && string(payload[1:7]) == "vorbis" {
		return "vorbis", nil
	}
	return "", errors.New("ogg: unrecognized codec")
}

func detectOGGCodecPath(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return detectOGGCodec(f)
}

func readDurationOGGVorbis(f *os.File) (time.Duration, error) {
	offset, err := skipID3v2(f)
	if err != nil {
		return 0, err
	}
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	r, err := oggvorbis.NewReader(io.NewSectionReader(f, offset, info.Size()-offset))
	if err != nil {
		return 0, fmt.Errorf("ogg/vorbis: %w", err)
	}
	if r.SampleRate() == 0 || r.Length() <= 0 {
		return 0, errors.New("ogg/vorbis: unknown length")
	}
	return toDuration(float64(r.Length()) / float64(r.SampleRate())), nil
}

func readDurationOpus(f *os.File) (time.Duration, error) {
	if _, err := skipID3v2(f); err != nil {
		return 0, err
	}

	var hdr [27]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return 0, err
	}
	if string(hdr[0:4]) != "OggS" {
		return 0, errors.New("opus: not an Ogg file")
	}
	segCount := int(hdr[26])
	segTable := make([]byte, segCount)
	if _, err := io.ReadFull(f, segTable); err != nil {
		return 0, err
	}
	payloadSize := 0
	for _, s := range segTable {
		payloadSize += int(s)
	}
	id := make([]byte, payloadSize)
	if _, err := io.ReadFull(f, id); err != nil {
		return 0, err
	}
	if len(id) < 19 || string(id[0:8]) != "OpusHead" {
		return 0, errors.New("opus: invalid ID header")
	}
	preSkip := uint64(binary.LittleEndian.Uint16(id[10:12]))

	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	scanStart := st.Size() - 65536
	if scanStart < 0 {
		scanStart = 0
	}
	if _, err := f.Seek(scanStart, io.SeekStart); err != nil {
		return 0, err
	}
	tail := make([]byte, st.Size()-scanStart)
	if _, err := io.ReadFull(f, tail); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return 0, err
	}

	lastGranule := uint64(0)
	for i := 0; i+27 < len(tail); i++ {
		if tail[i] != 'O' || tail[i+1] != 'g' || tail[i+2] != 'g' || tail[i+3] != 'S' {
			continue
		}
		g := binary.LittleEndian.Uint64(tail[i+6 : i+14])
		if g != 0xFFFFFFFFFFFFFFFF {
			lastGranule = g
		}
	}
	if lastGranule == 0 || lastGranule <= preSkip {
		return 0, errors.New("opus: invalid granule/preskip")
	}
	return toDuration(float64(lastGranule-preSkip) / 48000.0), nil
}

func readDurationSPC(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	var h [0xD0]byte
	if _, err := io.ReadFull(f, h[:]); err != nil {
		return 0, err
	}
	if string(h[0:27]) != "SNES-SPC700 Sound File Data" {
		return 0, errors.New("spc: not a valid SPC file")
	}
	if h[0x23] != 0x1A {
		return 0, errors.New("spc: no ID666 tag")
	}

	isBinary := false
	for _, b := range h[0xA0:0xA4] {
		if b != 0 && (b < 0x20 || b > 0x7E) {
			isBinary = true
			break
		}
	}
	if !isBinary {
		for _, b := range h[0x9E:0xA1] {
			if b != 0 && (b < '0' || b > '9') {
				isBinary = true
				break
			}
		}
	}

	seconds := 0.0
	if isBinary {
		ms := binary.LittleEndian.Uint32(h[0xA9 : 0xA9+4])
		seconds = float64(ms) / 1000
	} else {
		v := 0
		for _, c := range strings.TrimRight(string(h[0x9E:0xA1]), "\x00") {
			if c >= '0' && c <= '9' {
				v = v*10 + int(c-'0')
			}
		}
		seconds = float64(v)
	}

	if seconds <= 0 {
		//return readDurationGME(f.Name())
		return toDuration(0), nil
	}
	return toDuration(seconds), nil
}
