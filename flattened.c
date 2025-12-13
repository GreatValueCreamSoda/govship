#include "flattened.h"

Vship_Exception ComputeSSIMU2_flat(
    Vship_SSIMU2Handler *handler,
    double *score,
    uint8_t *s0, uint8_t *s1, uint8_t *s2,
    int64_t ls0, int64_t ls1, int64_t ls2,
    uint8_t *d0, uint8_t *d1, uint8_t *d2,
    int64_t ld0, int64_t ld1, int64_t ld2
) {
    const uint8_t* src[3] = { s0, s1, s2 };
    const uint8_t* dst[3] = { d0, d1, d2 };
    int64_t srcLine[3] = { ls0, ls1, ls2 };
    int64_t dstLine[3] = { ld0, ld1, ld2 };

    return Vship_ComputeSSIMU2(*handler, score, src, dst, srcLine, dstLine);
}

Vship_Exception ComputeButteraugli_flat(
    Vship_ButteraugliHandler *handler,
    Vship_ButteraugliScore* score,
    const uint8_t* dstp, int64_t dststride,
    const uint8_t* s0, const uint8_t* s1, const uint8_t* s2,
    const uint8_t* d0, const uint8_t* d1, const uint8_t* d2,
    int64_t ls0, int64_t ls1, int64_t ls2,
    int64_t ld0, int64_t ld1, int64_t ld2
) {
    const uint8_t* srcp1[3] = { s0, s1, s2 };
    const uint8_t* srcp2[3] = { d0, d1, d2 };
    const int64_t lineSize[3] = { ls0, ls1, ls2 };
    const int64_t lineSize2[3] = { ld0, ld1, ld2 };

    return Vship_ComputeButteraugli(
        *handler, score, dstp, dststride, srcp1, srcp2, lineSize, lineSize2);
}

Vship_Exception LoadTemporalCVVDP_flat(
    Vship_CVVDPHandler *handler,
    const uint8_t* s0, const uint8_t* s1, const uint8_t* s2,
    const uint8_t* d0, const uint8_t* d1, const uint8_t* d2,
    int64_t ls0, int64_t ls1, int64_t ls2,
    int64_t ld0, int64_t ld1, int64_t ld2
) {
    const uint8_t* srcp1[3] = { s0, s1, s2 };
    const uint8_t* srcp2[3] = { d0, d1, d2 };
    const int64_t lineSize[3] = { ls0, ls1, ls2 };
    const int64_t lineSize2[3] = { ld0, ld1, ld2 };
    return Vship_LoadTemporalCVVDP(*handler, srcp1, srcp2, lineSize, lineSize2);
}

Vship_Exception ComputeCVVDP_flat(
    Vship_CVVDPHandler *handler,
    double* score,
    const uint8_t* dstp, int64_t dststride,
    const uint8_t* s0, const uint8_t* s1, const uint8_t* s2,
    const uint8_t* d0, const uint8_t* d1, const uint8_t* d2,
    int64_t ls0, int64_t ls1, int64_t ls2,
    int64_t ld0, int64_t ld1, int64_t ld2
) {
    const uint8_t* srcp1[3] = { s0, s1, s2 };
    const uint8_t* srcp2[3] = { d0, d1, d2 };
    const int64_t lineSize[3] = { ls0, ls1, ls2 };
    const int64_t lineSize2[3] = { ld0, ld1, ld2 };
    return Vship_ComputeCVVDP(
        *handler, score, dstp, dststride, srcp1, srcp2, lineSize, lineSize2);
}