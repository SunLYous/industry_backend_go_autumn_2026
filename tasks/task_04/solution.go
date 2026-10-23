package main

import "math"

type Stats struct {
	Count         int
	Sum, Min, Max int64
}

func Calc(nums []int64) Stats {
	if len(nums) < 2 {
		return Stats{}
	}

	var s = Stats{
		Min: math.MaxInt64,
		Max: math.MinInt64,
	}

	s.Count = len(nums) - 1
	for i := 1; i < len(nums); i++ {
		d := nums[i] - nums[i-1]
		s.Sum += d

		if s.Min >= d {
			s.Min = d
		}

		if s.Max <= d {
			s.Max = d
		}
	}

	return s
}
