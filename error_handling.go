package govship

// #include "VshipAPI.h"
// #include <stdlib.h>
import "C"
import (
	"errors"
	"unsafe"
)

// ExceptionCode represents a status returned by Vship operations.
//
// It indicates whether an operation succeeded or failed, and if it failed,
// which category of error occurred. All Vship functions return an
// ExceptionCode to communicate success or failure.
type ExceptionCode int

// Predefined ExceptionCodes correspond to specific failure types returned
// by Vship operations, such as running out of memory, invalid inputs, or
// device errors.
//
// Users can compare returned ExceptionCodes against these constants to handle
// specific error cases.
const (
	ExceptionCodeNoError            ExceptionCode = C.Vship_NoError
	ExceptionCodeOutOfVRAM          ExceptionCode = C.Vship_OutOfVRAM
	ExceptionCodeOutOfRAM           ExceptionCode = C.Vship_OutOfRAM
	ExceptionCodeHIPError           ExceptionCode = C.Vship_HIPError
	ExceptionCodeBadDisplayModel    ExceptionCode = C.Vship_BadDisplayModel
	ExceptionCodeDifferingInputType ExceptionCode = C.Vship_DifferingInputType
	ExceptionCodeNonRGBSInput       ExceptionCode = C.Vship_NonRGBSInput
	ExceptionCodeBadPath            ExceptionCode = C.Vship_BadPath
	ExceptionCodeBadJson            ExceptionCode = C.Vship_BadJson
	ExceptionCodeDeviceCountError   ExceptionCode = C.Vship_DeviceCountError
	ExceptionCodeNoDeviceDetected   ExceptionCode = C.Vship_NoDeviceDetected
	ExceptionCodeBadDeviceArgument  ExceptionCode = C.Vship_BadDeviceArgument
	ExceptionCodeBadDeviceCode      ExceptionCode = C.Vship_BadDeviceCode
	ExceptionCodeBadHandler         ExceptionCode = C.Vship_BadHandler
	ExceptionCodeBadPointer         ExceptionCode = C.Vship_BadPointer
	ExceptionCodeBadErrorType       ExceptionCode = C.Vship_BadErrorType
)

// IsNone returns true if the operation completed successfully.
//
// It is the idiomatic way to check whether an ExceptionCode indicates no
// error.
func (e ExceptionCode) IsNone() bool { return e == ExceptionCodeNoError }

// GetError returns a human-readable description of the error.
//
// If the ExceptionCode represents a failure, this returns a descriptive Go
// error. If there was no error, the returned error string will be nil.
func (e ExceptionCode) GetError() error {
	var msgSize C.int = C.Vship_GetErrorMessage(C.Vship_Exception(e), nil, 0)
	var cPtr *C.char = (*C.char)(C.malloc(C.size_t(msgSize)))
	defer C.free(unsafe.Pointer(cPtr))
	C.Vship_GetErrorMessage(C.Vship_Exception(e), cPtr, msgSize)
	return errors.New(C.GoString(cPtr))
}
