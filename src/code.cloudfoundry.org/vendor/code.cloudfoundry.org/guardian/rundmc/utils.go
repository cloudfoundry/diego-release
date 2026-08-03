package rundmc

import (
	"math"
)

// reverse of runc cgroups.ConvertCPUSharesToCgroupV2Value
func ConvertCgroupV2ValueToCPUShares(cpuWeight uint64) uint64 {
	if cpuWeight == 0 {
		return 0
	}
	if cpuWeight <= 1 {
		return 2
	}
	if cpuWeight >= 10000 {
		return 262144
	}

	// Rebuild the targetValue from cpuWeight
	targetValue := math.Sqrt(float64(cpuWeight-1) * float64(cpuWeight))
	exponent := math.Log10(targetValue)

	// Reconstruct quadratic: l² + 125l - 612*(exponent + 7/34) = 0
	constant := 612.0 * (exponent + 7.0/34.0)
	discriminant := 125.0*125.0 + 4.0*constant
	l := (-125.0 + math.Sqrt(discriminant)) / 2.0
	// Now convert back to shares
	return uint64(math.Round(math.Pow(2, l)))

}
