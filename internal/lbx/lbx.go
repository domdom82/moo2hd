package lbx

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// RecordType classifies the content of an embedded record.
type RecordType int

const (
	RecordUnknown RecordType = iota
	RecordLBX                // nested binary / sprite / animation data
	RecordVOC                // Creative Voice sound effect (16-byte sub-header)
	RecordWAV                // RIFF/WAV audio (no sub-header)
	RecordXMI                // XMIDI music (16-byte sub-header)
	RecordSMK                // Smacker video (entire file; not a real LBX container)
)

func (r RecordType) String() string {
	switch r {
	case RecordLBX:
		return "LBX"
	case RecordVOC:
		return "VOC"
	case RecordWAV:
		return "WAV"
	case RecordXMI:
		return "XMI"
	case RecordSMK:
		return "SMK"
	default:
		return "unknown"
	}
}

var (
	magicLBX = [4]byte{0xAD, 0xFE, 0x00, 0x00}
	magicSMK = [4]byte{0x53, 0x4D, 0x4B, 0x32}
	magicVOC = [4]byte{0x43, 0x72, 0x65, 0x61}
	magicWAV = [4]byte{0x52, 0x49, 0x46, 0x46}
	magicXMI = [4]byte{0x46, 0x4F, 0x52, 0x4D}
	magicDrv = [4]byte{0x2D, 0x00, 0x43, 0x6F}
	magicMO2 = [4]byte{0x00, 0x08, 0x00, 0x00}
)

// vocAudioHeader is the size of the per-record sub-header in VOC and XMI archives.
const vocAudioHeader = 16

// filenameBlockBase is the file offset where the optional name/desc section begins.
const filenameBlockBase = 512

// Record is a single embedded file extracted from an LBX archive.
type Record struct {
	Index int
	Type  RecordType
	Name  string
	Desc  string
	// Data is the extracted payload, with any archive-level sub-header already stripped.
	Data []byte
}

// Archive is a parsed LBX file.
type Archive struct {
	// ArchiveType describes the content class of the whole archive.
	ArchiveType RecordType
	// IsMoO2 is true for Master of Orion 2 LBX files, which lack filename metadata.
	IsMoO2 bool
	Records []*Record
}

// Parse reads an LBX archive from raw bytes.
func Parse(data []byte) (*Archive, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("lbx: file too short (%d bytes)", len(data))
	}

	// SMK check must precede LBX magic — some .LBX files are renamed Smacker videos.
	if [4]byte(data[0:4]) == magicSMK {
		return &Archive{
			ArchiveType: RecordSMK,
			Records: []*Record{{
				Index: 0,
				Type:  RecordSMK,
				Data:  data,
			}},
		}, nil
	}

	if [4]byte(data[2:6]) != magicLBX {
		return nil, fmt.Errorf("lbx: invalid magic bytes at offset 2")
	}

	count := int(binary.LittleEndian.Uint16(data[0:2]))
	if count == 0 {
		return &Archive{}, nil
	}

	// Offset table: count × uint32LE starting at byte 8.
	minLen := 8 + count*4
	if len(data) < minLen {
		return nil, fmt.Errorf("lbx: file too short for offset table (need %d bytes, have %d)", minLen, len(data))
	}
	offsets := make([]int, count)
	for i := range count {
		pos := 8 + i*4
		offsets[i] = int(binary.LittleEndian.Uint32(data[pos : pos+4]))
	}

	isMoO2 := len(data) >= 12 && [4]byte(data[8:12]) == magicMO2

	// Detect archive-wide content type.
	archiveType := detectArchiveType(data, offsets)

	// Parse optional filename section (MoM/MoO1 only; absent in MoO2).
	names := make([]string, count)
	descs := make([]string, count)
	if !isMoO2 && offsets[0] != filenameBlockBase && len(data) >= filenameBlockBase+count*32 {
		for i := range count {
			base := filenameBlockBase + i*32
			names[i] = strings.TrimRight(string(data[base:base+8]), "\x00")
			descs[i] = strings.TrimRight(string(data[base+9:base+31]), "\x00")
		}
	}

	records := make([]*Record, count)
	for i := range count {
		start := offsets[i]
		end := len(data)
		if i < count-1 {
			end = offsets[i+1]
		}
		if start > len(data) || end > len(data) || start > end {
			return nil, fmt.Errorf("lbx: record %d has invalid offset range [%d, %d) in %d-byte file", i, start, end, len(data))
		}

		raw := data[start:end]
		recType, payload := extractRecord(archiveType, raw, i, count)

		records[i] = &Record{
			Index: i,
			Type:  recType,
			Name:  names[i],
			Desc:  descs[i],
			Data:  payload,
		}
	}

	return &Archive{
		ArchiveType: archiveType,
		IsMoO2:      isMoO2,
		Records:     records,
	}, nil
}

// detectArchiveType determines the content class of the whole archive by inspecting
// magic bytes at known positions in the first record.
func detectArchiveType(data []byte, offsets []int) RecordType {
	if len(offsets) == 0 {
		return RecordUnknown
	}

	first := offsets[0]

	// VOC and XMI have a 16-byte per-record sub-header; check payload at first+16.
	if first+vocAudioHeader+4 <= len(data) {
		sig := [4]byte(data[first+vocAudioHeader : first+vocAudioHeader+4])
		switch sig {
		case magicVOC:
			return RecordVOC
		case magicXMI:
			return RecordXMI
		}
	}

	if first+4 <= len(data) {
		sig := [4]byte(data[first : first+4])
		switch sig {
		case magicDrv:
			return RecordXMI // data+XMI: only last two records are music
		case magicWAV:
			return RecordWAV
		}
	}

	return RecordLBX
}

// extractRecord strips any archive-level sub-header and returns the record's
// content type and clean payload bytes.
func extractRecord(archiveType RecordType, raw []byte, i, total int) (RecordType, []byte) {
	switch archiveType {
	case RecordVOC:
		// Per-record 16-byte sub-header; check for embedded WAV first.
		if len(raw) >= 4 && [4]byte(raw[0:4]) == magicWAV {
			return RecordWAV, raw
		}
		if len(raw) > vocAudioHeader {
			return RecordVOC, raw[vocAudioHeader:]
		}
		return RecordVOC, raw

	case RecordXMI:
		// data+XMI: only last two records are music; others are opaque driver data.
		if i == total-2 || i == total-1 {
			if len(raw) > vocAudioHeader {
				return RecordXMI, raw[vocAudioHeader:]
			}
		}
		return RecordUnknown, raw

	case RecordWAV:
		// WAV archives may still contain per-record WAV or VOC.
		if len(raw) >= 4 {
			switch [4]byte(raw[0:4]) {
			case magicWAV:
				return RecordWAV, raw
			case magicVOC:
				if len(raw) > vocAudioHeader {
					return RecordVOC, raw[vocAudioHeader:]
				}
			}
		}
		return RecordUnknown, raw

	default:
		return RecordLBX, raw
	}
}
