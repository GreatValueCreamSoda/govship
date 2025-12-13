package govship_test

import (
	"fmt"
	"testing"

	vship "github.com/GreatValueCreamSoda/govship"
)

func Test_DisplayModelsToCVVDPJSON_Print(t *testing.T) {
	models := map[string]vship.DisplayModel{
		"new_Display_Name": {
			Name:                            "My HDR Monitor",
			ColorSpace:                      vship.DisplayModelColorspaceHDR,
			DisplayWidth:                    3840,
			DisplayHeight:                   2160,
			DisplayMaxLuminance:             1000.0,
			DisplayDiagonalSizeInches:       27.0,
			ViewingDistanceMeters:           0.7,
			MonitorContrastRatio:            1000,
			AmbientLightLevel:               10,
			AmbientLightReflectionOnDisplay: 0.01,
			Exposure:                        1.0,
		},
		"standard_fhd": {
			Name:       "override default colorspace for standard_fhd display",
			ColorSpace: vship.DisplayModelColorspaceSDR,
		},
	}

	jsonBytes, err := vship.DisplayModelsToCVVDPJSON(models)
	if err != nil {
		t.Fatalf("failed to generate JSON: %v", err)
	}

	fmt.Println(string(jsonBytes))
}
