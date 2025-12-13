package govship_test

import (
	"testing"

	vship "github.com/GreatValueCreamSoda/govship"
)

func Test_GetVersion(t *testing.T) {
	var version vship.Version = vship.GetVersion()
	var backend string
	if version.Backend == vship.BackendCuda {
		backend = "cuda"
	} else {
		backend = "hip"
	}

	t.Logf("Major: %d Minor: %d Minor Minor: %d Backend: %s", version.Major,
		version.Minor, version.MinorMinor, backend)
}

func Test_GetDeviceCount(t *testing.T) {
	var count int
	var exception vship.ExceptionCode
	count, exception = vship.GetDeviceCount()
	if !exception.IsNone() {
		t.Log(exception.GetError())
	}

	t.Logf("Num Devices: %d", count)
}

func Test_FullGpuCheck(t *testing.T) {
	var count int
	var exception vship.ExceptionCode
	count, exception = vship.GetDeviceCount()
	if !exception.IsNone() {
		t.Log(exception.GetError())
	}

	for i := range count {
		exception = vship.FullGpuCheck(i)
		if !exception.IsNone() {
			t.Log(exception.GetError())
		}
	}
}

func Test_SetDevice(t *testing.T) {
	var count int
	var exception vship.ExceptionCode
	count, exception = vship.GetDeviceCount()
	if !exception.IsNone() {
		t.Log(exception.GetError())
	}

	for i := range count {
		exception = vship.SetDevice(i)
		if !exception.IsNone() {
			t.Log(exception.GetError())
		}
	}
}

func Test_GetDeviceInfo(t *testing.T) {
	var count int
	var exception vship.ExceptionCode
	count, exception = vship.GetDeviceCount()
	if !exception.IsNone() {
		t.Log(exception.GetError())
	}

	for i := range count {
		var deviceInfo vship.DeviceInfo

		deviceInfo, exception = vship.GetDeviceInfo(i)
		if !exception.IsNone() {
			t.Log(exception.GetError())
		}

		t.Log(deviceInfo.GetString())
	}
}
