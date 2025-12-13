package govship

//#cgo LDFLAGS: -lvship
//#cgo CFLAGS: -I/usr/include -I./c
// #include <VshipAPI.h>
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"unsafe"
)

// Backend represents the Vship backend type.
type Backend int

const (
	BackendHIP  Backend = 0
	BackendCuda Backend = 1
)

// Version contains Vship library version info.
type Version struct {
	Major, Minor, MinorMinor int
	Backend                  Backend
}

// GetVersion returns the Vship library version.
func GetVersion() Version {
	v := C.Vship_GetVersion()
	return Version{
		int(v.major), int(v.minor), int(v.minorMinor), Backend(v.backend)}
}

func GetDeviceCount() (int, ExceptionCode) {
	var cPtr *C.int = (*C.int)(C.malloc(C.size_t(unsafe.Sizeof(C.int(0)))))
	defer C.free(unsafe.Pointer(cPtr))
	var code ExceptionCode = ExceptionCode(C.Vship_GetDeviceCount(cPtr))
	return int(*cPtr), code
}

func FullGpuCheck(gpuId int) ExceptionCode {
	return ExceptionCode(C.Vship_GPUFullCheck(C.int(gpuId)))
}

func SetDevice(gpuId int) ExceptionCode {
	return ExceptionCode(C.Vship_SetDevice(C.int(gpuId)))
}

// DeviceInfo contains information about a GPU device.
type DeviceInfo struct {
	Name                string
	VRAMSize            uint64
	Integrated          bool
	MultiProcessorCount int
	WarpSize            int
}

func (di *DeviceInfo) GetString() string {
	return fmt.Sprintf("Name: %s VramSize: %f GiB Integrated: %t"+
		" Processor Count: %d Warp Size: %d", di.Name,
		float64(di.VRAMSize)/1024/1024/1024, di.Integrated,
		di.MultiProcessorCount, di.WarpSize)
}

// GetDeviceInfo retrieves information about a GPU device.
func GetDeviceInfo(gpuID int) (DeviceInfo, ExceptionCode) {
	var deviceSize C.Vship_DeviceInfo
	var cPtr *C.Vship_DeviceInfo = (*C.Vship_DeviceInfo)(C.malloc(C.size_t(
		unsafe.Sizeof(deviceSize))))
	defer C.free(unsafe.Pointer(cPtr))
	var code ExceptionCode = ExceptionCode(C.Vship_GetDeviceInfo(cPtr,
		C.int(gpuID)))
	if !code.IsNone() {
		return DeviceInfo{}, code
	}

	return DeviceInfo{
		C.GoString(&cPtr.name[0]), uint64(cPtr.VRAMSize), cPtr.integrated != 0,
		int(cPtr.MultiProcessorCount), int(cPtr.WarpSize)}, code
}

// SSIMU2
