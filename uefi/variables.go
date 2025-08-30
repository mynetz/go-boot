// Copyright (c) The go-boot authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package uefi

import (
	"errors"
	"unicode/utf16"
)

// EFI Runtime Services offset for Variable Services
// See: https://uefi.org/specs/UEFI/2.11/08_Services_Runtime_Services.html#variable-services
const (
	getVariable         = 0x48
	getNextVariableName = 0x50
	setVariable         = 0x58
	queryVariableInfo   = 0x80

	EFI_GLOBAL_VARIABLE_GUID = "8BE4DF61-93CA-11D2-AA0D-00E098032B8C"
)

var (
	// TODO: Move to proper place
	ErrEfiNotFound = errors.New("not found")
)

// VariableAttributes represents the attributes of a UEFI variable.
// See: https://uefi.org/specs/UEFI/2.11/08_Services_Runtime_Services.html#getvariable
type VariableAttributes struct {
	NonVolatile              bool
	BootServiceAccess        bool
	RuntimeServiceAccess     bool
	HardwareErrorRecord      bool
	AuthWriteAccess          bool
	TimeBasedAuthWriteAccess bool
	AppendWrite              bool
	EnhancedAuthAccess       bool
}

// stringToUTF16 converts a Go string to null-terminated UTF-16 (CHAR16) bytes
func stringToUTF16(s string) []byte {
	utf16Codes := utf16.Encode([]rune(s))
	// Add null terminator
	utf16Codes = append(utf16Codes, 0)

	// TODO: Check if this is correct. Shouldn't the above be enough??

	// Convert to little-endian byte array
	buf := make([]byte, len(utf16Codes)*2)
	for i, code := range utf16Codes {
		buf[i*2] = byte(code & 0xff)
		buf[i*2+1] = byte(code >> 8)
	}
	return buf
}

// utf16BytesToString converts UTF-16 bytes back to Go string
func utf16BytesToString(buf []byte) string {
	if len(buf)%2 != 0 {
		return ""
	}

	codes := make([]uint16, len(buf)/2)
	for i := 0; i < len(codes); i++ {
		codes[i] = uint16(buf[i*2]) | (uint16(buf[i*2+1]) << 8)
	}

	// Find null terminator
	for i, code := range codes {
		if code == 0 {
			codes = codes[:i]
			break
		}
	}

	return string(utf16.Decode(codes))
}

// GetVariable calls EFI_RUNTIME_SERVICES.GetVariable().
// See: https://uefi.org/specs/UEFI/2.11/08_Services_Runtime_Services.html#getvariable
func (s *RuntimeServices) GetVariable(variableName string, vendorGuid []byte, withData bool) (attr VariableAttributes, dataSize uint64, data []byte, err error) {
	// Convert lastName to UTF-16 for UEFI
	nameUTF16 := stringToUTF16(variableName)

	var attributes uint32

	// The first call retrieves the attributes and size of data
	status := callService(s.base+getVariable,
		[]uint64{
			ptrval(&nameUTF16[0]),
			ptrval(&vendorGuid[0]),
			ptrval(&attributes),
			ptrval(&dataSize),
			0,
		},
	)

	if status != EFI_SUCCESS && status&0xff != EFI_BUFFER_TOO_SMALL {
		err = parseStatus(status)
		return VariableAttributes{}, 0, nil, err
	}

	// Parse attributes
	attr.NonVolatile = attributes&0x1 != 0
	attr.BootServiceAccess = attributes&0x2 != 0
	attr.RuntimeServiceAccess = attributes&0x4 != 0
	attr.HardwareErrorRecord = attributes&0x8 != 0
	attr.AuthWriteAccess = attributes&0x10 != 0
	attr.TimeBasedAuthWriteAccess = attributes&0x20 != 0
	attr.AppendWrite = attributes&0x40 != 0
	attr.EnhancedAuthAccess = attributes&0x80 != 0

	if !withData {
		return attr, dataSize, nil, nil
	}

	// The second call retrieves the data
	data = make([]byte, dataSize)
	status = callService(s.base+getVariable,
		[]uint64{
			ptrval(&nameUTF16[0]),
			ptrval(&vendorGuid[0]),
			0,
			ptrval(&dataSize),
			ptrval(&data[0]),
		},
	)

	if err = parseStatus(status); err != nil {
		return attr, 0, nil, err
	}

	return attr, dataSize, data, nil
}

// GetNextVariableName calls EFI_RUNTIME_SERVICES.GetNextVariableName().
// See: https://uefi.org/specs/UEFI/2.11/08_Services_Runtime_Services.html#getnextvariablename
func (s *RuntimeServices) GetNextVariableName(lastName string, lastGuid []byte) (variableName string, vendorGuid []byte, err error) {
	// Convert lastName to UTF-16 for UEFI
	lastNameUTF16 := stringToUTF16(lastName)

	// TODO: Change lastGuid to byte array!!! The use the outer function to convert user input etc. into the right format
	// 	e.g. user provides "8BE4DF61-93CA-11D2-AA0D-00E098032B8C"
	// 	lastGuid := GUID("..").Bytes() is used then
	// 	don't know if we need to convert it back to GUID string format

	// Calculate buffer size: need space for variable name (UTF-16) + null terminator
	// UEFI spec suggests 1024 bytes minimum, but we need more for longer names
	initialSize := uint64(1024)
	requiredSize := uint64(len(lastNameUTF16))
	if requiredSize > initialSize {
		initialSize = requiredSize
	}

	// Create buffer that can hold UTF-16 encoded variable names
	nameBuf := make([]byte, initialSize)
	copy(nameBuf, lastNameUTF16)

	// Prepare GUID buffer - use lastGuid bytes directly
	guidBuf := make([]byte, 16) // GUID is always 16 bytes
	if lastName != "" {
		copy(guidBuf, lastGuid)
	} else {
		// For first call with empty lastName, initialize with zeros
		// UEFI will return the first variable's GUID
	}

	status := callService(s.base+getNextVariableName,
		[]uint64{
			ptrval(&initialSize),
			ptrval(&nameBuf[0]),
			ptrval(&guidBuf[0]),
		},
	)

	err = parseStatus(status)
	if err != nil {
		if status&0xff == EFI_NOT_FOUND {
			err = ErrEfiNotFound
		} else {
			return "", nil, err
		}
	}

	// Convert returned UTF-16 name back to Go string
	variableName = utf16BytesToString(nameBuf)

	return variableName, guidBuf, err
}

// SetVariable calls EFI_RUNTIME_SERVICES.SetVariable().
// See: https://uefi.org/specs/UEFI/2.11/08_Services_Runtime_Services.html#setvariable
//func (s *RuntimeServices) SetVariable(name string, guid *GUID, attr uint32, size uint32, data []byte) (err error) {
//
//}

// QueryVariableInfo calls EFI_RUNTIME_SERVICES.QueryVariableInfo().
// See: https://uefi.org/specs/UEFI/2.11/08_Services_Runtime_Services.html#queryvariableinfo
//func (s *RuntimeServices) QueryVariableInfo(attr uint32, size *uint32, data []byte) (err error) {
//
//}
