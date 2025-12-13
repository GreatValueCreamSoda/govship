package govship

// #include <stdint.h>
import "C"
import "unsafe"

// safeMalloc is a wrapper around the C-level malloc function.
// The size of element to allocate is passed as type T and numElements
// represents the number of elements of size T to allocate.
func safeMalloc[T any](numElements uint) *T {
	var size T
	var bytes C.size_t = C.size_t(unsafe.Sizeof(size) * uintptr(numElements))
	var ptr unsafe.Pointer = C.malloc(bytes)
	return (*T)(ptr)
}

// helper to get a C pointer for a plane (or nil)
func planePtr(b []byte) *C.uint8_t {
	if len(b) == 0 {
		return nil
	}
	return (*C.uint8_t)(unsafe.Pointer(&b[0]))
}
