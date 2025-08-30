// Copyright (c) The go-boot authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package uefi

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
)

var guidPattern = regexp.MustCompile(`^([[:xdigit:]]{8})-([[:xdigit:]]{4})-([[:xdigit:]]{4})-([[:xdigit:]]{4})-([[:xdigit:]]{12})$`)

// GUID represents an EFI GUID (Globally Unique Identifier).
type GUID string

// GuidFromBytes converts a 16-byte representation back to GUID registry format
func GuidFromBytes(buf []byte) GUID {
	if len(buf) != 16 {
		return ""
	}

	return GUID(fmt.Sprintf("%08x-%04x-%04x-%x-%x",
		binary.LittleEndian.Uint32(buf[0:4]),
		binary.LittleEndian.Uint16(buf[4:6]),
		binary.LittleEndian.Uint16(buf[6:8]),
		buf[8:10],
		buf[10:]))
}

// Bytes returns the GUID as byte slice.
func (g GUID) Bytes() (guid []byte) {
	var buf []byte
	var err error

	m := guidPattern.FindStringSubmatch(string(g))

	if len(m) != 6 {
		return make([]byte, 16)
	}

	m = m[1:]

	for i, b := range m {
		if buf, err = hex.DecodeString(b); err != nil {
			return make([]byte, 16)
		}

		switch i {
		case 0:
			guid = append(guid, buf[3])
			guid = append(guid, buf[2])
			guid = append(guid, buf[1])
			guid = append(guid, buf[0])
		case 1, 2:
			guid = append(guid, buf[1])
			guid = append(guid, buf[0])
		default:
			guid = append(guid, buf...)
		}
	}

	return
}

func (g GUID) ptrval() uint64 {
	buf := g.Bytes()
	return ptrval(&buf[0])
}
