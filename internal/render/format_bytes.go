package render

import (
	"fmt"
	"math"
)

func FormatBytes(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
	}
	abs := math.Abs(float64(n))
	if abs < 1024 {
		return fmt.Sprintf("%s%d B", sign, int64(abs))
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	value := abs
	unit := units[0]
	for _, u := range units {
		value /= 1024
		unit = u
		if value < 1024 {
			break
		}
	}
	return fmt.Sprintf("%s%.1f %s", sign, value, unit)
}
