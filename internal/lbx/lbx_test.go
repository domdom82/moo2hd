package lbx_test

import (
	"encoding/binary"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/lbx"
)

// buildArchive constructs a minimal LBX file in memory.
//
// records: raw payloads for each embedded file.
// isMoO2: if true, writes the MoO2 sentinel at bytes 8-11.
// withNames: if true and !isMoO2, writes an 8-byte name + 22-byte desc per record
// at offset 512 (only valid when the first record starts after offset 512).
func buildArchive(records [][]byte, isMoO2 bool, names, descs []string) []byte {
	count := len(records)

	// Offset table starts at byte 8; each entry is 4 bytes.
	// First record begins right after the offset table.
	firstOffset := 8 + count*4

	// If we want a filename section the first record must start after byte 512.
	if len(names) > 0 && !isMoO2 {
		if firstOffset < 512+count*32 {
			firstOffset = 512 + count*32
		}
	}

	// Compute total size.
	offsets := make([]int, count)
	pos := firstOffset
	for i, rec := range records {
		offsets[i] = pos
		pos += len(rec)
	}
	totalSize := pos

	buf := make([]byte, totalSize)

	// Header: count (2), magic (4), content-info (2).
	binary.LittleEndian.PutUint16(buf[0:2], uint16(count))
	copy(buf[2:6], []byte{0xAD, 0xFE, 0x00, 0x00})
	binary.LittleEndian.PutUint16(buf[6:8], 0)

	// Offset table.
	for i, off := range offsets {
		pos := 8 + i*4
		if isMoO2 {
			// MoO2 sentinel: first 4 bytes of offset table are 00 08 00 00.
			// This means we must put the sentinel at bytes 8-11, overwriting
			// the first offset entry. The actual first record offset is NOT
			// encoded in those bytes for MoO2 files in the reference impl —
			// the sentinel itself serves as the detection flag.
			// We still write all offsets; the sentinel check only reads bytes 8-11.
			_ = i
		}
		binary.LittleEndian.PutUint32(buf[pos:pos+4], uint32(off))
	}

	// MoO2 sentinel overwrites bytes 8-11.
	if isMoO2 {
		copy(buf[8:12], []byte{0x00, 0x08, 0x00, 0x00})
		// Restore the actual first offset after byte 11 would corrupt things,
		// so instead just set isMoO2 and trust Parse() to detect it.
		// In real MoO2 files the first record offset IS 0x0800 = 2048.
		// For our test fixture we make the first record start at 2048.
	}

	// Filename section (non-MoO2 only).
	if len(names) > 0 && !isMoO2 {
		for i := 0; i < count && i < len(names); i++ {
			base := 512 + i*32
			name := names[i]
			if len(name) > 8 {
				name = name[:8]
			}
			copy(buf[base:base+8], []byte(name))
			desc := ""
			if i < len(descs) {
				desc = descs[i]
			}
			if len(desc) > 22 {
				desc = desc[:22]
			}
			copy(buf[base+9:base+31], []byte(desc))
		}
	}

	// Record payloads.
	for i, rec := range records {
		copy(buf[offsets[i]:], rec)
	}

	return buf
}

// smkFile returns a minimal Smacker video byte slice.
func smkFile() []byte {
	b := make([]byte, 32)
	copy(b[0:4], []byte{0x53, 0x4D, 0x4B, 0x32})
	return b
}

// wavPayload returns a minimal RIFF/WAV payload.
func wavPayload() []byte {
	b := make([]byte, 8)
	copy(b[0:4], []byte{0x52, 0x49, 0x46, 0x46})
	return b
}

// vocPayload returns a VOC payload preceded by a 16-byte sub-header.
func vocPayload() []byte {
	b := make([]byte, 20)
	copy(b[16:20], []byte{0x43, 0x72, 0x65, 0x61}) // VOC magic at +16
	return b
}

// xmiPayload returns an XMI payload preceded by a 16-byte sub-header.
func xmiPayload() []byte {
	b := make([]byte, 20)
	copy(b[16:20], []byte{0x46, 0x4F, 0x52, 0x4D}) // XMI magic at +16
	return b
}

var _ = Describe("LBX Archive", func() {
	Describe("Parse", func() {
		Context("error cases", func() {
			It("rejects files shorter than 8 bytes", func() {
				_, err := lbx.Parse([]byte{0x01, 0x02})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("too short"))
			})

			It("rejects files with wrong magic bytes", func() {
				bad := make([]byte, 16)
				copy(bad[2:6], []byte{0xDE, 0xAD, 0xBE, 0xEF})
				_, err := lbx.Parse(bad)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid magic"))
			})

			It("rejects archives whose offset table extends past EOF", func() {
				// count=10 needs 8+40=48 bytes minimum; give only 20.
				buf := make([]byte, 20)
				binary.LittleEndian.PutUint16(buf[0:2], 10)
				copy(buf[2:6], []byte{0xAD, 0xFE, 0x00, 0x00})
				_, err := lbx.Parse(buf)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("SMK detection", func() {
			It("returns a single SMK record without parsing LBX structure", func() {
				arc, err := lbx.Parse(smkFile())
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.ArchiveType).To(Equal(lbx.RecordSMK))
				Expect(arc.Records).To(HaveLen(1))
				Expect(arc.Records[0].Type).To(Equal(lbx.RecordSMK))
			})
		})

		Context("empty archive", func() {
			It("returns an archive with no records", func() {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint16(buf[0:2], 0)
				copy(buf[2:6], []byte{0xAD, 0xFE, 0x00, 0x00})
				arc, err := lbx.Parse(buf)
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.Records).To(BeEmpty())
			})
		})

		Context("MoO2 plain binary archive", func() {
			var arc *lbx.Archive
			payload1 := []byte{0x01, 0x02, 0x03, 0x04}
			payload2 := []byte{0xAA, 0xBB}
			payload3 := []byte{0xFF}

			BeforeEach(func() {
				// Build a 3-record archive; first offset at 8+3*4=20.
				raw := buildArchive([][]byte{payload1, payload2, payload3}, false, nil, nil)
				var err error
				arc, err = lbx.Parse(raw)
				Expect(err).NotTo(HaveOccurred())
			})

			It("has three records", func() {
				Expect(arc.Records).To(HaveLen(3))
			})

			It("extracts each record's payload correctly", func() {
				Expect(arc.Records[0].Data).To(Equal(payload1))
				Expect(arc.Records[1].Data).To(Equal(payload2))
				Expect(arc.Records[2].Data).To(Equal(payload3))
			})

			It("classifies records as LBX type", func() {
				for _, r := range arc.Records {
					Expect(r.Type).To(Equal(lbx.RecordLBX))
				}
			})

			It("reports archive type as LBX", func() {
				Expect(arc.ArchiveType).To(Equal(lbx.RecordLBX))
			})
		})

		Context("last record boundary", func() {
			It("reads the last record through EOF", func() {
				payload := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE}
				raw := buildArchive([][]byte{[]byte{0x00}, payload}, false, nil, nil)
				arc, err := lbx.Parse(raw)
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.Records[1].Data).To(Equal(payload))
			})
		})

		Context("WAV archive", func() {
			It("detects archive type as WAV and returns payload directly", func() {
				raw := buildArchive([][]byte{wavPayload()}, false, nil, nil)
				arc, err := lbx.Parse(raw)
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.ArchiveType).To(Equal(lbx.RecordWAV))
				Expect(arc.Records[0].Type).To(Equal(lbx.RecordWAV))
				Expect(arc.Records[0].Data[:4]).To(Equal([]byte{0x52, 0x49, 0x46, 0x46}))
			})
		})

		Context("VOC archive", func() {
			It("detects archive type as VOC and strips the 16-byte sub-header", func() {
				raw := buildArchive([][]byte{vocPayload()}, false, nil, nil)
				arc, err := lbx.Parse(raw)
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.ArchiveType).To(Equal(lbx.RecordVOC))
				Expect(arc.Records[0].Type).To(Equal(lbx.RecordVOC))
				// After stripping the 16-byte header the VOC magic should be at byte 0.
				Expect(arc.Records[0].Data[:4]).To(Equal([]byte{0x43, 0x72, 0x65, 0x61}))
			})

			It("recognises embedded WAV records within a VOC archive", func() {
				// VOC archive where record 0 is VOC and record 1 is a bare WAV.
				vocRaw := buildArchive([][]byte{vocPayload(), wavPayload()}, false, nil, nil)
				arc, err := lbx.Parse(vocRaw)
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.ArchiveType).To(Equal(lbx.RecordVOC))
				Expect(arc.Records[1].Type).To(Equal(lbx.RecordWAV))
			})
		})

		Context("XMI archive", func() {
			It("detects archive type and strips the 16-byte sub-header", func() {
				raw := buildArchive([][]byte{xmiPayload()}, false, nil, nil)
				arc, err := lbx.Parse(raw)
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.ArchiveType).To(Equal(lbx.RecordXMI))
				Expect(arc.Records[0].Type).To(Equal(lbx.RecordXMI))
				Expect(arc.Records[0].Data[:4]).To(Equal([]byte{0x46, 0x4F, 0x52, 0x4D}))
			})
		})

		Context("filename section (non-MoO2)", func() {
			It("reads name and desc for each record when the section is present", func() {
				p1 := make([]byte, 4)
				p2 := make([]byte, 4)
				raw := buildArchive(
					[][]byte{p1, p2},
					false,
					[]string{"SHIP", "PLANET"},
					[]string{"Fighter description", "Colony world"},
				)
				arc, err := lbx.Parse(raw)
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.Records[0].Name).To(Equal("SHIP"))
				Expect(arc.Records[0].Desc).To(Equal("Fighter description"))
				Expect(arc.Records[1].Name).To(Equal("PLANET"))
				Expect(arc.Records[1].Desc).To(Equal("Colony world"))
			})

			It("leaves names empty when first record starts at the filename block offset", func() {
				// First offset == 512 means filename section is absent.
				// Build manually: count=1, first record at 512.
				buf := make([]byte, 516)
				binary.LittleEndian.PutUint16(buf[0:2], 1)
				copy(buf[2:6], []byte{0xAD, 0xFE, 0x00, 0x00})
				binary.LittleEndian.PutUint32(buf[8:12], 512) // first record AT 512
				buf[512] = 0xAB
				arc, err := lbx.Parse(buf)
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.Records[0].Name).To(BeEmpty())
			})
		})

		Context("MoO2 sentinel", func() {
			It("sets IsMoO2 = true and produces empty names", func() {
				// Build archive: the test helper writes MoO2 sentinel at bytes 8-11.
				// For a valid MoO2 archive the first offset value IS 0x0800 (2048).
				buf := make([]byte, 2100)
				binary.LittleEndian.PutUint16(buf[0:2], 1)
				copy(buf[2:6], []byte{0xAD, 0xFE, 0x00, 0x00})
				// Write MoO2 sentinel — this also IS the first offset entry.
				copy(buf[8:12], []byte{0x00, 0x08, 0x00, 0x00}) // offset = 2048
				buf[2048] = 0x42
				arc, err := lbx.Parse(buf)
				Expect(err).NotTo(HaveOccurred())
				Expect(arc.IsMoO2).To(BeTrue())
				Expect(arc.Records[0].Name).To(BeEmpty())
				Expect(arc.Records[0].Data[0]).To(Equal(byte(0x42)))
			})
		})
	})

	Describe("RecordType.String", func() {
		DescribeTable("returns the expected label",
			func(rt lbx.RecordType, expected string) {
				Expect(rt.String()).To(Equal(expected))
			},
			Entry("LBX", lbx.RecordLBX, "LBX"),
			Entry("VOC", lbx.RecordVOC, "VOC"),
			Entry("WAV", lbx.RecordWAV, "WAV"),
			Entry("XMI", lbx.RecordXMI, "XMI"),
			Entry("SMK", lbx.RecordSMK, "SMK"),
			Entry("unknown", lbx.RecordUnknown, "unknown"),
		)
	})
})
