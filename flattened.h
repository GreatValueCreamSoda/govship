#ifndef SSIMU2_FLATTENED_H
#define SSIMU2_FLATTENED_H

#include <stdint.h>
#include <VshipAPI.h>

/*
 * vship_flattened.h
 *
 * Why this exists:
 * The original Vship functions use arrays of pointers (like srcp1[3]) and
 * arrays of line sizes. Go’s cgo rules don’t allow passing Go slice memory
 * inside arrays of pointers directly to C, because the garbage collector could
 * move them and break things. That’s why calls like the ones in the VshipAPI
 * header usually produce runtime errors.
 *
 * How do these functions fix this?:
 * They “flatten” those arrays into separate arguments for each plane and line
 * size. The wrapper builds the arrays on the C stack internally, so Go only
 * ever passes simple pointers to its slice memory — fully safe and zero-copy.
*/

Vship_Exception ComputeSSIMU2_flat(
    Vship_SSIMU2Handler *handler,
    double *score,
    uint8_t *s0, uint8_t *s1, uint8_t *s2,
    int64_t ls0, int64_t ls1, int64_t ls2,
    uint8_t *d0, uint8_t *d1, uint8_t *d2,
    int64_t ld0, int64_t ld1, int64_t ld2
);

Vship_Exception ComputeButteraugli_flat(
    Vship_ButteraugliHandler *handler,
    Vship_ButteraugliScore* score,
    const uint8_t* dstp,
    int64_t dststride,
    const uint8_t* s0, const uint8_t* s1, const uint8_t* s2,
    const uint8_t* d0, const uint8_t* d1, const uint8_t* d2,
    int64_t ls0, int64_t ls1, int64_t ls2,
    int64_t ld0, int64_t ld1, int64_t ld2
);

Vship_Exception LoadTemporalCVVDP_flat(
    Vship_CVVDPHandler *handler,
    const uint8_t* s0, const uint8_t* s1, const uint8_t* s2,
    const uint8_t* d0, const uint8_t* d1, const uint8_t* d2,
    int64_t ls0, int64_t ls1, int64_t ls2,
    int64_t ld0, int64_t ld1, int64_t ld2
);

Vship_Exception ComputeCVVDP_flat(
    Vship_CVVDPHandler *handler,
    double* score,
    const uint8_t* dstp,
    int64_t dststride,
    const uint8_t* s0, const uint8_t* s1, const uint8_t* s2,
    const uint8_t* d0, const uint8_t* d1, const uint8_t* d2,
    int64_t ls0, int64_t ls1, int64_t ls2,
    int64_t ld0, int64_t ld1, int64_t ld2
);

#endif // SSIMU2_FLATTENED_H