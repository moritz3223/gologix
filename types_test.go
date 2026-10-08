package gologix

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestGetBitSINT locks in that getBit accepts the int8 that readValue produces
// for a SINT. Before the fix getBit asserted byte, so every SINT bit read
// failed with a misleading "must be 0-7" error (issue #71).
func TestGetBitSINT(t *testing.T) {
	cases := []struct {
		raw    byte
		bitpos int
		want   bool
	}{
		{0x08, 3, true},
		{0x08, 2, false},
		{0x80, 7, true}, // sign bit of a negative int8
		{0x7F, 7, false},
	}
	for _, c := range cases {
		v, err := readValue(CIPTypeSINT, bytes.NewReader([]byte{c.raw}))
		if err != nil {
			t.Fatalf("readValue(SINT 0x%02X): %v", c.raw, err)
		}
		got, err := getBit(CIPTypeSINT, v, c.bitpos)
		if err != nil {
			t.Errorf("getBit(SINT 0x%02X, bit %d): %v", c.raw, c.bitpos, err)
			continue
		}
		if got != c.want {
			t.Errorf("getBit(SINT 0x%02X, bit %d) = %v; want %v", c.raw, c.bitpos, got, c.want)
		}
	}
}

// TestGetBitOutOfRange: a bit position outside the type's width must be an
// error. Returning (false, nil) is indistinguishable from a bit that is
// genuinely off, so a typo in a tag address became good data (issue #71).
func TestGetBitOutOfRange(t *testing.T) {
	cases := []struct {
		name   string
		typ    CIPType
		v      any
		bitpos int
	}{
		{"BOOL bit 1", CIPTypeBOOL, true, 1},
		{"SINT bit 8", CIPTypeSINT, int8(-1), 8},
		{"BYTE bit -1", CIPTypeBYTE, byte(0xFF), -1},
		{"INT bit 16", CIPTypeINT, int16(-1), 16},
		{"DINT bit 40", CIPTypeDINT, int32(-1), 40},
		{"UDINT bit 32", CIPTypeUDINT, uint32(0xFFFFFFFF), 32},
		{"LINT bit 64", CIPTypeLINT, int64(-1), 64},
		{"LWORD bit -1", CIPTypeLWORD, uint64(1), -1},
	}
	for _, c := range cases {
		got, err := getBit(c.typ, c.v, c.bitpos)
		if err == nil {
			t.Errorf("%s: got (%v, nil); want error", c.name, got)
		}
	}
}

// TestGetBitWrongGoType keeps the existing contract: a value whose Go type
// does not match the CIP type is still an error, not a silent false.
func TestGetBitWrongGoType(t *testing.T) {
	if _, err := getBit(CIPTypeDINT, int16(1), 0); err == nil {
		t.Fatal("DINT with an int16 value: want error, got nil")
	}
}

// TestCIPTypeString checks the human readable names for the known CIP types,
// including the ones that used to fall through to the generic Unknown message.
func TestCIPTypeString(t *testing.T) {
	cases := map[CIPType]string{
		CIPTypeUnknown:         "0x00 - Unknown",
		CIPTypeStruct:          "0xA0 - Struct",
		CIPTypeUTIME:           "0xC0 - UTIME",
		CIPTypeBOOL:            "0xC1 - BOOL",
		CIPTypeSINT:            "0xC2 - SINT",
		CIPTypeINT:             "0xC3 - INT",
		CIPTypeDINT:            "0xC4 - DINT",
		CIPTypeLINT:            "0xC5 - LINT",
		CIPTypeUSINT:           "0xC6 - USINT",
		CIPTypeUINT:            "0xC7 - UINT",
		CIPTypeUDINT:           "0xC8 - UDINT",
		CIPTypeULINT:           "0xC9 - ULINT",
		CIPTypeREAL:            "0xCA - REAL",
		CIPTypeLREAL:           "0xCB - LREAL",
		CIPTypeSTIME:           "0xCC - STIME",
		CIPTypeDATE:            "0xCD - DATE",
		CIPTypeTIMEOFDAY:       "0xCE - TIME_OF_DAY",
		CIPTypeDATETIME:        "0xCF - DATE_AND_TIME",
		CIPTypeSTRING_UNKNOWN:  "0xD0 - STRINGI",
		CIPTypeBYTE:            "0xD1 - BYTE",
		CIPTypeWORD:            "0xD2 - WORD",
		CIPTypeDWORD:           "0xD3 - DWORD",
		CIPTypeLWORD:           "0xD4 - LWORD",
		CIPTypeSTRING_UNKNOWN2: "0xD5 - STRING2",
		CIPTypeFTIME:           "0xD6 - FTIME",
		CIPTypeLTIME:           "0xD7 - LTIME",
		CIPTypeITIME:           "0xD8 - ITIME",
		CIPTypeSTRING_UNKNOWN3: "0xD9 - STRINGN",
		CIPTypeSTRING_SHORT:    "0xDA - SHORT_STRING",
		CIPTypeTIMEOFDAY2:      "0xDB - TIME",
		CIPTypeEPATH:           "0xDC - EPATH",
		CIPTypeENGUNIT:         "0xDD - ENGUNIT",
		CIPTypeSTRING:          "0xFF - (gologix specific) String",
	}
	for cipType, want := range cases {
		if got := cipType.String(); got != want {
			t.Errorf("CIPType(0x%02X).String() = %q; want %q", byte(cipType), got, want)
		}
	}
	if got := CIPType(0x42).String(); !strings.Contains(got, "Unknown") {
		t.Errorf("unlisted type should print as Unknown, got %q", got)
	}
}

// TestCIPTypeSize checks the byte size of every known atomic CIP type.
func TestCIPTypeSize(t *testing.T) {
	cases := map[CIPType]int{
		CIPTypeBOOL:       1,
		CIPTypeSINT:       1,
		CIPTypeUSINT:      1,
		CIPTypeBYTE:       1,
		CIPTypeINT:        2,
		CIPTypeUINT:       2,
		CIPTypeWORD:       2,
		CIPTypeDATE:       2,
		CIPTypeENGUNIT:    2,
		CIPTypeDINT:       4,
		CIPTypeUDINT:      4,
		CIPTypeDWORD:      4,
		CIPTypeREAL:       4,
		CIPTypeSTIME:      4,
		CIPTypeFTIME:      4,
		CIPTypeITIME:      4,
		CIPTypeTIMEOFDAY2: 4,
		CIPTypeTIMEOFDAY:  6,
		CIPTypeLINT:       8,
		CIPTypeULINT:      8,
		CIPTypeLWORD:      8,
		CIPTypeLREAL:      8,
		CIPTypeUTIME:      8,
		CIPTypeLTIME:      8,
		CIPTypeDATETIME:   8,
	}
	for cipType, want := range cases {
		if got := cipType.Size(); got != want {
			t.Errorf("%v.Size() = %d; want %d", cipType, got, want)
		}
	}
}

// TestReadValueNewTypes checks that the previously unsupported types decode
// to the expected Go values.
func TestReadValueNewTypes(t *testing.T) {
	cases := []struct {
		name string
		typ  CIPType
		raw  []byte
		want any
	}{
		{"ULINT", CIPTypeULINT, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, uint64(0xFFFFFFFFFFFFFFFF)},
		{"UTIME", CIPTypeUTIME, []byte{0x40, 0xE2, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00}, int64(123456)},
		{"LTIME", CIPTypeLTIME, []byte{0x40, 0xE2, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00}, int64(123456)},
		{"DATE_AND_TIME", CIPTypeDATETIME, []byte{0x40, 0xE2, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00}, int64(123456)},
		{"STIME", CIPTypeSTIME, []byte{0x10, 0x27, 0x00, 0x00}, int32(10000)},
		{"FTIME", CIPTypeFTIME, []byte{0x10, 0x27, 0x00, 0x00}, int32(10000)},
		{"ITIME", CIPTypeITIME, []byte{0x10, 0x27, 0x00, 0x00}, int32(10000)},
		{"TIME", CIPTypeTIMEOFDAY2, []byte{0x10, 0x27, 0x00, 0x00}, int32(10000)},
		{"DATE", CIPTypeDATE, []byte{0xB1, 0x4E}, uint16(20145)},
		{"ENGUNIT", CIPTypeENGUNIT, []byte{0x41, 0x09}, uint16(2369)},
		{"TIME_OF_DAY", CIPTypeTIMEOFDAY, []byte{1, 2, 3, 4, 5, 6}, [6]byte{1, 2, 3, 4, 5, 6}},
		{"EPATH", CIPTypeEPATH, []byte{0x02, 0x00, 0x20, 0x6B, 0x24, 0x01}, []byte{0x02, 0x00, 0x20, 0x6B, 0x24, 0x01}},
		{"STRINGN", CIPTypeSTRING_UNKNOWN3, []byte{0x03, 0x00, 0x00, 0x00, 'a', 'b', 'c'}, []byte{0x03, 0x00, 0x00, 0x00, 'a', 'b', 'c'}},
		{"SHORT_STRING", CIPTypeSTRING_SHORT, []byte{0x03, 'a', 'b', 'c'}, []byte{0x03, 'a', 'b', 'c'}},
	}
	for _, c := range cases {
		got, err := readValue(c.typ, bytes.NewReader(c.raw))
		if err != nil {
			t.Errorf("%s: readValue error: %v", c.name, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: readValue = %#v; want %#v", c.name, got, c.want)
		}
	}
}

// TestGetBitNewTypes checks bit access on the newly supported wide types.
func TestGetBitNewTypes(t *testing.T) {
	cases := []struct {
		name   string
		typ    CIPType
		v      any
		bitpos int
		want   bool
	}{
		{"ULINT bit 63", CIPTypeULINT, uint64(1) << 63, 63, true},
		{"ULINT bit 0", CIPTypeULINT, uint64(0), 0, false},
		{"UTIME bit 8", CIPTypeUTIME, int64(0x100), 8, true},
		{"LTIME bit 8", CIPTypeLTIME, int64(0x100), 8, true},
		{"DATE_AND_TIME bit 8", CIPTypeDATETIME, int64(0x100), 8, true},
		{"STIME bit 4", CIPTypeSTIME, int32(0x10), 4, true},
		{"DATE bit 0", CIPTypeDATE, uint16(1), 0, true},
	}
	for _, c := range cases {
		got, err := getBit(c.typ, c.v, c.bitpos)
		if err != nil {
			t.Errorf("%s: getBit error: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: getBit = %v; want %v", c.name, got, c.want)
		}
	}
}

// TestReadSINTBitAccess drives the user-visible path for issue #71:
// Read("Tag.3", *bool) against a SINT tag goes through readValue then getBit.
func TestReadSINTBitAccess(t *testing.T) {
	client, fs := newFakeCIPClient(t)

	done := make(chan error, 1)
	var got bool
	go func() {
		done <- client.Read("MySint.3", &got)
	}()

	req := fs.awaitRequest(time.Second)
	if req.service != CIPService_Read {
		t.Fatalf("expected service Read, got %v", req.service)
	}
	fs.replyConnectedRead(CIPService_Read, req.seq, 0x00, CIPTypeSINT, []byte{0x08})

	if err := <-done; err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if !got {
		t.Fatal("bit 3 of SINT 0x08 read as false; want true")
	}
}

// TestReadMultiOutOfRangeBit covers the batch path. readList used to log and
// skip a getBit error, leaving a nil result that ReadMulti then handed to
// reflect.Value.Set, which panics. An out-of-range bit position must come back
// from ReadMulti as an error.
func TestReadMultiOutOfRangeBit(t *testing.T) {
	client, fs := newFakeCIPClient(t)

	type tags struct {
		Bit bool `gologix:"MyInt.20"`
	}
	done := make(chan error, 1)
	go func() {
		var got tags
		done <- client.ReadMulti(&got)
	}()

	req := fs.awaitRequest(time.Second)
	var data bytes.Buffer
	_ = binary.Write(&data, binary.LittleEndian, int16(-1))
	fs.replyMultiRead(req.seq, [][]byte{multiReadReplyBody(CIPTypeINT, 0x00, data.Bytes())})

	err := <-done
	if err == nil {
		t.Fatal("ReadMulti of bit 20 on an INT: want error, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("want the out-of-range error from getBit, got: %v", err)
	}
}
