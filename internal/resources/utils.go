package resources

import "math"

const (
	thousand = 1000
	million  = 1000 * 1000
	billion  = 1000 * 1000 * 1000

	gigabyte = int64(1024 * 1024 * 1024)
)

func roundUpGB(bytes int64) int64 {
	return int64(math.Ceil(float64(bytes)/float64(gigabyte))) * gigabyte
}
