package avflow

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// agoraUIDMax is the upper bound for numeric Agora uids (2^32-1).
const agoraUIDMax = uint32(4294967295)

// agoraAccountMaxLen is the max user-account string length per Agora token APIs.
const agoraAccountMaxLen = 255

type agoraUIDKind uint8

const (
	agoraUIDUnset agoraUIDKind = iota
	agoraUIDNumber
	agoraUIDString
)

// AgoraUID is the local Agora identity on the wire (numeric uid or user account).
// JSON accepts a number or string; omit or 0 lets Agora assign a numeric uid.
type AgoraUID struct {
	kind agoraUIDKind
	num  uint32
	str  string
}

// NewAgoraUIDNum builds a numeric uid (0 is treated as unset).
func NewAgoraUIDNum(n uint32) AgoraUID {
	if n == 0 {
		return AgoraUID{}
	}
	return AgoraUID{kind: agoraUIDNumber, num: n}
}

// NewAgoraUIDAccount builds a string user account uid.
func NewAgoraUIDAccount(account string) AgoraUID {
	return AgoraUID{kind: agoraUIDString, str: account}
}

func (u AgoraUID) IsUnset() bool { return u.kind == agoraUIDUnset }

func (u AgoraUID) IsNumeric() bool { return u.kind == agoraUIDNumber }

func (u AgoraUID) IsString() bool { return u.kind == agoraUIDString }

// Num returns the numeric uid (0 when unset or account mode).
func (u AgoraUID) Num() uint32 {
	if u.kind != agoraUIDNumber {
		return 0
	}
	return u.num
}

// Account returns the string user account ("" when unset or numeric mode).
func (u AgoraUID) Account() string {
	if u.kind != agoraUIDString {
		return ""
	}
	return u.str
}

func (u AgoraUID) MarshalJSON() ([]byte, error) {
	switch u.kind {
	case agoraUIDUnset:
		return []byte("null"), nil
	case agoraUIDNumber:
		return json.Marshal(u.num)
	case agoraUIDString:
		return json.Marshal(u.str)
	default:
		return nil, fmt.Errorf("agora uid: invalid kind %d", u.kind)
	}
}

func (u *AgoraUID) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*u = AgoraUID{}
		return nil
	}
	var num uint32
	if err := json.Unmarshal(data, &num); err == nil {
		if num == 0 {
			*u = AgoraUID{}
			return nil
		}
		if num > agoraUIDMax {
			return fmt.Errorf("agora uid: numeric uid exceeds %d", agoraUIDMax)
		}
		*u = AgoraUID{kind: agoraUIDNumber, num: num}
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str == "" {
			return fmt.Errorf("agora uid: empty string")
		}
		if len(str) > agoraAccountMaxLen {
			return fmt.Errorf("agora uid: account exceeds %d bytes", agoraAccountMaxLen)
		}
		*u = AgoraUID{kind: agoraUIDString, str: str}
		return nil
	}
	return fmt.Errorf("agora uid: want number or string, got %s", data)
}
