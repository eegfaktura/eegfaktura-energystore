package utils

import "math"

func Sum(sl []float64) float64 {
	var sum float64 = float64(0)
	for _, v := range sl {
		sum += v
	}
	return sum
}

func RoundToFixed(value float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(value*ratio) / ratio
}

func InitSlice(value int, s []int) []int {
	for i := range s {
		s[i] = value
	}
	return s
}
