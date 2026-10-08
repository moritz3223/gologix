package gologix

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type CIPType byte

// Go native types that correspond to logix types
// I'm not sure whether having interface here makes sense.
// On the one hand, we need to support composite types, but on the other this lets it accept anything
// which doesn't seem right.
type GoLogixTypes interface {
	bool | byte | int8 | uint16 | int16 | uint32 | int32 | uint64 | int64 | float32 | float64 | string
}

// return the CIPType that corresponds to go type of variable T
// also return the element count
func GoVarToCIPType(T any) (CIPType, int) {
	switch x := T.(type) {
	case bool, *bool:
		return CIPTypeBOOL, 1
	case byte, *byte:
		return CIPTypeBYTE, 1
	case int8, *int8:
		return CIPTypeSINT, 1
	case uint16, *uint16:
		return CIPTypeUINT, 1
	case int16, *int16:
		return CIPTypeINT, 1
	case uint32, *uint32:
		return CIPTypeUDINT, 1
	case int32, *int32:
		return CIPTypeDINT, 1
	case uint64, *uint64:
		return CIPTypeLWORD, 1
	case int64, *int64:
		return CIPTypeLINT, 1
	case float32, *float32:
		return CIPTypeREAL, 1
	case float64, *float64:
		return CIPTypeLREAL, 1
	case string, *string:
		return CIPTypeSTRING, 1
	case []byte:
		return CIPTypeBYTE, len(x)
	case []int8:
		return CIPTypeSINT, len(x)
	case []uint16:
		return CIPTypeUINT, len(x)
	case []int16:
		return CIPTypeINT, len(x)
	case []uint32:
		return CIPTypeUDINT, len(x)
	case []int32:
		return CIPTypeDINT, len(x)
	case []uint64:
		return CIPTypeLWORD, len(x)
	case []int64:
		return CIPTypeLINT, len(x)
	case []float32:
		return CIPTypeREAL, len(x)
	case []float64:
		return CIPTypeLREAL, len(x)
	case []string:
		return CIPTypeSTRING, len(x)
	case interface{}:
		return CIPTypeStruct, 1
	}
	return CIPTypeUnknown, 1
}

const (
	CIPTypeUnknown         CIPType = 0x00
	CIPTypeStruct          CIPType = 0xA0 // also used for strings.  Not sure what's up with CIPTypeSTRING
	CIPTypeUTIME           CIPType = 0xC0
	CIPTypeBOOL            CIPType = 0xC1
	CIPTypeSINT            CIPType = 0xC2
	CIPTypeINT             CIPType = 0xC3
	CIPTypeDINT            CIPType = 0xC4
	CIPTypeLINT            CIPType = 0xC5
	CIPTypeUSINT           CIPType = 0xC6
	CIPTypeUINT            CIPType = 0xC7
	CIPTypeUDINT           CIPType = 0xC8
	CIPTypeULINT           CIPType = 0xC9
	CIPTypeREAL            CIPType = 0xCA
	CIPTypeLREAL           CIPType = 0xCB
	CIPTypeSTIME           CIPType = 0xCC
	CIPTypeDATE            CIPType = 0xCD
	CIPTypeTIMEOFDAY       CIPType = 0xCE
	CIPTypeDATETIME        CIPType = 0xCF // DATE_AND_TIME
	CIPTypeSTRING_UNKNOWN  CIPType = 0xD0 // STRINGI
	CIPTypeBYTE            CIPType = 0xD1 // 8 bits packed into one byte
	CIPTypeWORD            CIPType = 0xD2
	CIPTypeDWORD           CIPType = 0xD3
	CIPTypeLWORD           CIPType = 0xD4
	CIPTypeSTRING_UNKNOWN2 CIPType = 0xD5 // STRING2
	CIPTypeFTIME           CIPType = 0xD6
	CIPTypeLTIME           CIPType = 0xD7
	CIPTypeITIME           CIPType = 0xD8
	CIPTypeSTRING_UNKNOWN3 CIPType = 0xD9 // STRINGN
	CIPTypeSTRING_SHORT    CIPType = 0xDA // SHORT_STRING
	CIPTypeTIMEOFDAY2      CIPType = 0xDB // TIME (milliseconds since midnight)
	CIPTypeEPATH           CIPType = 0xDC
	CIPTypeENGUNIT         CIPType = 0xDD

	//  Strings actually come accross as 0xA0 = CIPTypeStruct.
	//In this library we're using this as kind of a flag to keep track of whether
	// a structure is a normal logix string or not.
	CIPTypeSTRING CIPType = 0xFF
)

// Aliases for CIP types using the names found in the CIP specification and
// Logix documentation.
const (
	CIPTypeDATE_AND_TIME = CIPTypeDATETIME        // 0xCF
	CIPTypeTIME_OF_DAY   = CIPTypeTIMEOFDAY       // 0xCE
	CIPTypeTIME          = CIPTypeTIMEOFDAY2      // 0xDB
	CIPTypeSTRINGI       = CIPTypeSTRING_UNKNOWN  // 0xD0
	CIPTypeSTRING2       = CIPTypeSTRING_UNKNOWN2 // 0xD5
	CIPTypeSTRINGN       = CIPTypeSTRING_UNKNOWN3 // 0xD9
	CIPTypeSHORT_STRING  = CIPTypeSTRING_SHORT    // 0xDA
)

// return the size in bytes of the data structure
func (c CIPType) Size() int {
	switch c {
	case CIPTypeBOOL, CIPTypeSINT, CIPTypeUSINT, CIPTypeBYTE, CIPTypeSTRING:
		return 1
	case CIPTypeINT, CIPTypeUINT, CIPTypeWORD, CIPTypeDATE, CIPTypeENGUNIT:
		return 2
	case CIPTypeDINT, CIPTypeUDINT, CIPTypeDWORD, CIPTypeREAL,
		CIPTypeSTIME, CIPTypeFTIME, CIPTypeITIME, CIPTypeTIMEOFDAY2:
		return 4
	case CIPTypeTIMEOFDAY:
		return 6
	case CIPTypeLINT, CIPTypeULINT, CIPTypeLWORD, CIPTypeLREAL,
		CIPTypeUTIME, CIPTypeLTIME, CIPTypeDATETIME:
		return 8
	case CIPTypeStruct:
		return 88
	case CIPTypeUnknown:
		return 0
	default:
		return 0
	}
}

// return a buffer that can hold the data structure
func (c CIPType) NewBuffer() *[]byte {
	buf := make([]byte, c.Size())
	return &buf
}

// Sanity caps for the length-prefixed wire types (EPATH, STRINGI, STRING2,
// STRINGN). Their lengths come straight off the wire, so they are bounded
// before being used as allocation sizes. The caps are far above anything a
// real controller will send in a single tag value.
const (
	cipMaxLenPrefixedWords = 8192
	cipMaxLenPrefixedBytes = 1 << 20
)

// human readable version of the cip type for printing.
func (c CIPType) String() string {
	switch c {
	case CIPTypeUnknown:
		return "0x00 - Unknown"
	case CIPTypeStruct:
		return "0xA0 - Struct"
	case CIPTypeUTIME:
		return "0xC0 - UTIME"
	case CIPTypeBOOL:
		return "0xC1 - BOOL"
	case CIPTypeSINT:
		return "0xC2 - SINT"
	case CIPTypeINT:
		return "0xC3 - INT"
	case CIPTypeDINT:
		return "0xC4 - DINT"
	case CIPTypeLINT:
		return "0xC5 - LINT"
	case CIPTypeUSINT:
		return "0xC6 - USINT"
	case CIPTypeUINT:
		return "0xC7 - UINT"
	case CIPTypeUDINT:
		return "0xC8 - UDINT"
	case CIPTypeULINT:
		return "0xC9 - ULINT"
	case CIPTypeREAL:
		return "0xCA - REAL"
	case CIPTypeLREAL:
		return "0xCB - LREAL"
	case CIPTypeSTIME:
		return "0xCC - STIME"
	case CIPTypeDATE:
		return "0xCD - DATE"
	case CIPTypeTIMEOFDAY:
		return "0xCE - TIME_OF_DAY"
	case CIPTypeDATETIME:
		return "0xCF - DATE_AND_TIME"
	case CIPTypeSTRING_UNKNOWN:
		return "0xD0 - STRINGI"
	case CIPTypeBYTE:
		return "0xD1 - BYTE"
	case CIPTypeWORD:
		return "0xD2 - WORD"
	case CIPTypeDWORD:
		return "0xD3 - DWORD"
	case CIPTypeLWORD:
		return "0xD4 - LWORD"
	case CIPTypeSTRING_UNKNOWN2:
		return "0xD5 - STRING2"
	case CIPTypeFTIME:
		return "0xD6 - FTIME"
	case CIPTypeLTIME:
		return "0xD7 - LTIME"
	case CIPTypeITIME:
		return "0xD8 - ITIME"
	case CIPTypeSTRING_UNKNOWN3:
		return "0xD9 - STRINGN"
	case CIPTypeSTRING_SHORT:
		return "0xDA - SHORT_STRING"
	case CIPTypeTIMEOFDAY2:
		return "0xDB - TIME"
	case CIPTypeEPATH:
		return "0xDC - EPATH"
	case CIPTypeENGUNIT:
		return "0xDD - ENGUNIT"
	case CIPTypeSTRING:
		return "0xFF - (gologix specific) String"
	default:
		return fmt.Sprintf("0x%02x - Unknown", byte(c))
	}
}

func (t CIPType) IsAtomic() bool {
	v := byte(t)
	return v <= 254
}
func (t CIPType) readValue(r io.Reader) (any, error) {
	return readValue(t, r)
}

// readValue reads one unit of cip data type t into the correct go type.
// To do this it reads the needed number of bytes from r.
// It returns the value as an any so the caller will have to do a cast to get it back
func readValue(t CIPType, r io.Reader) (any, error) {

	var value any
	var err error
	switch t {
	case CIPTypeUnknown:
		return nil, fmt.Errorf("unknown type")
	case CIPTypeStruct:
		return nil, fmt.Errorf("don't know what to do with a struct")
	case CIPTypeBOOL:
		var trueval bool
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeBYTE:
		var trueval byte
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeSINT:
		var trueval int8
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeINT:
		var trueval int16
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeDINT:
		var trueval int32
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeLINT:
		var trueval int64
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeUSINT:
		var trueval uint8
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeUINT:
		var trueval uint16
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeUDINT:
		var trueval uint32
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeULINT:
		var trueval uint64
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeLWORD:
		var trueval uint64
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeREAL:
		var trueval float32
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeLREAL:
		var trueval float64
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeWORD:
		var trueval uint16
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeDWORD:
		var trueval uint32
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeUTIME, CIPTypeLTIME, CIPTypeDATETIME:
		// 8 byte counts: microseconds for UTIME, LTIME and DATE_AND_TIME
		var trueval int64
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeSTIME, CIPTypeFTIME, CIPTypeITIME, CIPTypeTIMEOFDAY2:
		// 4 byte counts: milliseconds for STIME, FTIME and TIME (TIMEOFDAY2)
		var trueval int32
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeDATE:
		// days since 1970-01-01 (unsigned per CIP spec)
		var trueval uint16
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeENGUNIT:
		// engineering unit code
		var trueval uint16
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeTIMEOFDAY:
		// 6 byte TIME_OF_DAY: 4 byte nanoseconds-of-day + 2 byte date (days)
		var trueval [6]byte
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	case CIPTypeEPATH:
		// EPATH is length-prefixed: number of 16-bit words in the path.
		// The word count comes off the wire so cap it before allocating.
		var words uint16
		err = binary.Read(r, binary.LittleEndian, &words)
		if err != nil {
			return nil, fmt.Errorf("problem reading EPATH length. %w", err)
		}
		if words > cipMaxLenPrefixedWords {
			return nil, fmt.Errorf("EPATH word count %d exceeds sanity limit %d", words, cipMaxLenPrefixedWords)
		}
		trueval := make([]byte, 2+2*int(words))
		binary.LittleEndian.PutUint16(trueval, words)
		_, err = io.ReadFull(r, trueval[2:])
		value = trueval
	case CIPTypeSTRING_UNKNOWN, CIPTypeSTRING_UNKNOWN2, CIPTypeSTRING_UNKNOWN3:
		// STRINGI, STRING2 and STRINGN: 32-bit char count followed by chars.
		// The count comes off the wire so cap it before allocating.
		var strlen uint32
		err = binary.Read(r, binary.LittleEndian, &strlen)
		if err != nil {
			return nil, fmt.Errorf("problem reading string length. %w", err)
		}
		if strlen > cipMaxLenPrefixedBytes {
			return nil, fmt.Errorf("string length %d exceeds sanity limit %d", strlen, cipMaxLenPrefixedBytes)
		}
		trueval := make([]byte, 4+strlen)
		binary.LittleEndian.PutUint32(trueval, strlen)
		_, err = io.ReadFull(r, trueval[4:])
		value = trueval
	case CIPTypeSTRING_SHORT:
		// SHORT_STRING: 8-bit char count followed by chars
		var strlen uint8
		err = binary.Read(r, binary.LittleEndian, &strlen)
		if err != nil {
			return nil, fmt.Errorf("problem reading short string length. %w", err)
		}
		trueval := make([]byte, 1+strlen)
		trueval[0] = strlen
		_, err = io.ReadFull(r, trueval[1:])
		value = trueval
	case CIPTypeSTRING:
		var trueval [86]byte
		err = binary.Read(r, binary.LittleEndian, &trueval)
		value = trueval
	default:
		return nil, fmt.Errorf("default (unknown) type %d", t)
		//panic(fmt.Sprintf("Default type %d", t))

	}
	if err != nil {
		return nil, fmt.Errorf("problem reading %s as one unit of %T. %w", t, value, err)
	}
	return value, nil
}

// after reading a value v from the controller, you can get a bit from it with
// getBit. bitpos must be between 0 and the length of the CIPType you read.
func getBit(t CIPType, v any, bitpos int) (bool, error) {

	switch t {
	case CIPTypeUnknown:
		return false, errors.New("unknown type")
	case CIPTypeStruct:
		return false, errors.New("got a struct - can't get a bit")
	case CIPTypeBOOL:
		if bitpos == 0 {
			x, ok := v.(bool)
			if ok {
				return x, nil
			}
			return false, fmt.Errorf("value was a bool, but bit %d was requested. must be 0 for bool", bitpos)
		}
	case CIPTypeBYTE, CIPTypeUSINT:
		if bitpos >= 0 && bitpos < 8 {
			x, ok := v.(byte)
			if ok {
				return x&(1<<bitpos) != 0, nil
			}
			return false, fmt.Errorf("value was a %v, but bit %d was requested. must be 0-7 for %v", t, bitpos, t)
		}
	case CIPTypeSINT:
		if bitpos >= 0 && bitpos < 8 {
			// readValue decodes a SINT as int8, not byte.
			x, ok := v.(int8)
			if ok {
				return byte(x)&(1<<bitpos) != 0, nil
			}
			return false, fmt.Errorf("value was a SINT, but bit %d was requested. must be 0-7 for SINT", bitpos)
		}
	case CIPTypeINT:
		if bitpos >= 0 && bitpos < 16 {
			x, ok := v.(int16)
			if ok {
				return x&(1<<bitpos) != 0, nil
			}
			return false, fmt.Errorf("value was an INT, but bit %d was requested. must be 0-15 for INT", bitpos)
		}
	case CIPTypeDINT:
		if bitpos >= 0 && bitpos < 32 {
			x, ok := v.(int32)
			if ok {
				return x&(1<<bitpos) != 0, nil
			}
			return false, fmt.Errorf("value was a DINT, but bit %d was requested. must be 0-31 for DINT", bitpos)
		}
	case CIPTypeLINT:
		if bitpos >= 0 && bitpos < 64 {
			x, ok := v.(int64)
			if ok {
				return x&(1<<bitpos) != 0, nil
			}
			return false, fmt.Errorf("value was a LINT, but bit %d was requested. must be 0-63 for LINT", bitpos)
		}
	case CIPTypeUINT, CIPTypeWORD, CIPTypeDATE, CIPTypeENGUNIT:
		if bitpos >= 0 && bitpos < 16 {
			x, ok := v.(uint16)
			if ok {
				return x&(1<<bitpos) != 0, nil
			}
			return false, fmt.Errorf("value was a %v, but bit %d was requested. must be 0-15 for %v", t, bitpos, t)
		}
	case CIPTypeUDINT, CIPTypeDWORD, CIPTypeSTIME, CIPTypeFTIME, CIPTypeITIME, CIPTypeTIMEOFDAY2:
		if bitpos >= 0 && bitpos < 32 {
			// readValue decodes the 32-bit time types as int32 and the rest as uint32.
			switch x := v.(type) {
			case uint32:
				return x&(1<<bitpos) != 0, nil
			case int32:
				return x&(1<<bitpos) != 0, nil
			}
			return false, fmt.Errorf("value was a %v, but bit %d was requested. must be 0-31 for %v", t, bitpos, t)
		}
	case CIPTypeULINT, CIPTypeLWORD, CIPTypeUTIME, CIPTypeLTIME, CIPTypeDATETIME:
		if bitpos >= 0 && bitpos < 64 {
			// readValue decodes the 64-bit time types as int64 and the rest as uint64.
			switch x := v.(type) {
			case uint64:
				return x&(1<<bitpos) != 0, nil
			case int64:
				return x&(1<<bitpos) != 0, nil
			}
			return false, fmt.Errorf("value was a %v, but bit %d was requested. must be 0-63 for %v", t, bitpos, t)
		}
	case CIPTypeREAL, CIPTypeLREAL:
		return false, fmt.Errorf("value was a %v, not finding bit of real", t)
	case CIPTypeSTRING:
		return false, fmt.Errorf("value was a STRING, not finding bit of string")
	case CIPTypeTIMEOFDAY,
		CIPTypeEPATH,
		CIPTypeSTRING_UNKNOWN, CIPTypeSTRING_UNKNOWN2,
		CIPTypeSTRING_UNKNOWN3, CIPTypeSTRING_SHORT:
		return false, fmt.Errorf("value was a %v, not finding bit of it", t)
	default:
		return false, errors.New("got an unknown type. don't know how to get bit")

	}
	// Every case above either returns or falls through on a failed bitpos guard,
	// so reaching here means the requested bit is out of range for the type.
	return false, fmt.Errorf("bit %d is out of range for %v", bitpos, t)
}
