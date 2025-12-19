package govship

import (
	"encoding/json"
	"fmt"
	"os"
)

// DisplayModelColorspace describes the perceptual colorspace a display
// operates in from CVVDP’s point of view.
//
// This is not a transfer function by itself; it selects a predefined
// perceptual pipeline inside the metric.
type DisplayModelColorspace string

const (
	// DisplayModelColorspaceHDR indicates a high dynamic range display
	// with perceptual behavior consistent with HDR viewing conditions.
	DisplayModelColorspaceHDR DisplayModelColorspace = "HDR"

	// DisplayModelColorspaceSDR indicates a standard dynamic range
	// display operating under SDR / sRGB-like assumptions.
	DisplayModelColorspaceSDR DisplayModelColorspace = "SDR"
)

// DisplayModel describes the physical and viewing characteristics of a display
// as seen by a human observer.
//
// These parameters are used by perceptual metrics such as CVVDP  to model
// visual sensitivity, contrast masking, and adaptation. Values should reflect
// real viewing conditions rather than arbitrary display specifications.
type DisplayModel struct {
	// Human-readable description of the display and viewing setup.
	Name string
	// Perceptual colorspace of the display (HDR or SDR). This is used as
	// nothing more than an internal weight in the pipeline. In general
	// DisplayModelColorspaceHDR should be used
	ColorSpace DisplayModelColorspace
	// Native resolution of the display in pixels.
	DisplayWidth, DisplayHeight int
	// Maximum display luminance in cd/m² (nits). The average SDR display will
	// vary between 150 to 500 nits peak brightness. HDR displays can vary
	// greatly in brightness between displays and content conditions. EX:
	// hotspots of 1500 nits with full display white only being 500 to 700.
	DisplayMaxLuminance float32
	// Physical diagonal size of the display in inches. Generally speaking the
	// smaller in size a screen is the harder it is for viwers to resolve
	// smaller details. Especially important when calculating scores intended
	// for phone viewing or other non standard desktop senarios.
	DisplayDiagonalSizeInches float32
	// Distance between the viewer and the display in meters. Generally
	// speaking the further away a viewer is from a screen the harder it is for
	// them to resolve smaller detail. Especially important when calculating
	// scores intended for phone viewing or other non standard desktop
	// senarios.
	ViewingDistanceMeters float32
	// Native contrast ratio of the display (e.g. 1000 for 1000:1). Typically
	// OLED monitors will have a "infinite" constrast ratio, but should be
	// properly specified as 1 million or 1 billion to 1.
	MonitorContrastRatio int
	// Ambient illumination level in lux. Living rooms can vary between 100 to
	// 200, bedrooms 200 to 500, and kitchens 250 to 1000 lux.
	AmbientLightLevel int
	// Fraction of ambient light reflected by the display surface. where 1 is
	// 100%
	AmbientLightReflectionOnDisplay float32
	// Exposure is a global perceptual scaling factor. This should almost
	// always be set to 1, matching CVVDP reference conditions.
	Exposure float32
}

// Built-in display presets corresponding to common CVVDP reference conditions.
// These are suitable defaults for most use cases and produce results
// comparable to published CVVDP benchmarks.
var (
	// DisplayModelPresetStandard4K models a typical SDR 4K monitor viewed
	// under office lighting conditions.
	DisplayModelPresetStandard4K DisplayModel = DisplayModel{
		Name: "30-inch 4K monitor, peak luminance 200 cd/m^2, viewed under " +
			"office light levels (250 lux), seen from 2 x display height",
		ColorSpace:                      DisplayModelColorspaceSDR,
		DisplayWidth:                    3840,
		DisplayHeight:                   2160,
		DisplayMaxLuminance:             200,
		DisplayDiagonalSizeInches:       30,
		ViewingDistanceMeters:           0.7472,
		MonitorContrastRatio:            1000,
		AmbientLightLevel:               250,
		AmbientLightReflectionOnDisplay: 0.005,
		Exposure:                        1,
	}

	// DisplayModelPresetStandardFHD models a typical SDR Full HD display under
	// the same viewing conditions as the 4K preset.
	DisplayModelPresetStandardFHD DisplayModel = DisplayModel{
		Name: "30-inch 4K monitor, peak luminance 200 cd/m^2, viewed under " +
			"office light levels (250 lux), seen from 2 x display height",
		ColorSpace:                      DisplayModelColorspaceSDR,
		DisplayWidth:                    1920,
		DisplayHeight:                   1080,
		DisplayMaxLuminance:             200,
		DisplayDiagonalSizeInches:       30,
		ViewingDistanceMeters:           0.7472,
		MonitorContrastRatio:            1000,
		AmbientLightLevel:               250,
		AmbientLightReflectionOnDisplay: 0.005,
		Exposure:                        1,
	}

	// DisplayModelPresetStandardHDR models a bright HDR monitor viewed in a
	// low-light environment.
	DisplayModelPresetStandardHDR DisplayModel = DisplayModel{
		Name: "30-inch 4K HDR monitor, peak luminance 1500 cd/m^2, viewed " +
			"under low light levels (10 lux), seen from 2 x display height",
		ColorSpace:                      DisplayModelColorspaceHDR,
		DisplayWidth:                    3840,
		DisplayHeight:                   2160,
		DisplayMaxLuminance:             1500,
		DisplayDiagonalSizeInches:       30,
		ViewingDistanceMeters:           0.7472,
		MonitorContrastRatio:            1000000,
		AmbientLightLevel:               10,
		AmbientLightReflectionOnDisplay: 0.005,
		Exposure:                        1,
	}

	// DisplayModelPresetStandardHDRDarkRoom models an HDR display viewed in a
	// near-dark environment with minimal ambient light.
	DisplayModelPresetStandardHDRDarkRoom DisplayModel = DisplayModel{
		Name: "30-inch 4K HDR monitor, peak luminance 1500 cd/m^2, viewed " +
			"under low light levels (10 lux), seen from 2 x display height",
		ColorSpace:                      DisplayModelColorspaceHDR,
		DisplayWidth:                    3840,
		DisplayHeight:                   2160,
		DisplayMaxLuminance:             1500,
		DisplayDiagonalSizeInches:       30,
		ViewingDistanceMeters:           0.7472,
		MonitorContrastRatio:            1000000,
		AmbientLightLevel:               0,
		AmbientLightReflectionOnDisplay: 0.005,
		Exposure:                        1,
	}
)

// cvvdpDisplayJSON is an internal representation matching the JSON schema
// expected by CVVDP display model configuration files.
//
// This type is not exported and should not be relied upon directly.
type cvvdpDisplayJSON struct {
	Name                  string  `json:"name,omitempty"`
	ColorSpace            string  `json:"colorspace,omitempty"`
	Resolution            [2]int  `json:"resolution,omitempty"`
	MaxLuminance          float32 `json:"max_luminance,omitempty"`
	ViewingDistanceMeters float32 `json:"viewing_distance_meters,omitempty"`
	DiagonalSizeInches    float32 `json:"diagonal_size_inches,omitempty"`
	Contrast              float32 `json:"contrast,omitempty"`
	EAmbient              float32 `json:"E_ambient,omitempty"`
	KRefl                 float32 `json:"k_refl,omitempty"`
	Exposure              float32 `json:"exposure,omitempty"`
	Source                string  `json:"source,omitempty"`
}

// DisplayModelsToCVVDPJSON converts a set of DisplayModel definitions into a
// CVVDP-compatible JSON configuration.
//
// The input map key becomes the display identifier in the JSON file. The
// resulting JSON can be written directly to disk and passed to
// NewCVVDPHandlerWithConfig as a custom display model configuration file for
// custom display model presets.
func DisplayModelsToCVVDPJSON(models []DisplayModel) ([]byte, error) {
	out := make(map[string]cvvdpDisplayJSON)

	for i, m := range models {
		key := m.Name
		if key == "" {
			// fallback if Name is empty
			key = fmt.Sprintf("display-%d", i)
		}
		out[key] = cvvdpDisplayJSON{
			Name:                  m.Name,
			ColorSpace:            string(m.ColorSpace),
			Resolution:            [2]int{m.DisplayWidth, m.DisplayHeight},
			MaxLuminance:          m.DisplayMaxLuminance,
			ViewingDistanceMeters: m.ViewingDistanceMeters,
			DiagonalSizeInches:    m.DisplayDiagonalSizeInches,
			Contrast:              float32(m.MonitorContrastRatio),
			EAmbient:              float32(m.AmbientLightLevel),
			KRefl:                 m.AmbientLightReflectionOnDisplay,
			Exposure:              m.Exposure,
			Source:                "none",
		}
	}

	return json.MarshalIndent(out, "", "    ")
}

// DisplayModelsToCVVDPJSONFile writes a set of DisplayModel definitions
// directly to a JSON file at the given path. Returns an error if marshalling
// or writing fails.
func DisplayModelsToCVVDPJSONFile(models []DisplayModel, filePath string,
) error {
	data, err := DisplayModelsToCVVDPJSON(models)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}
