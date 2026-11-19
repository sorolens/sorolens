package format

import "strings"

// Sparkline blocks, lowest to highest. They are all one cell wide in every
// common terminal font, so the line stays aligned next to text.
var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// Sparkline renders values as a unicode sparkline of at most width cells.
// Longer input is downsampled per bucket by its maximum (peaks survive,
// rather than being averaged away). Empty input, or a width of zero or less,
// renders an em dash so a missing column degrades instead of shifting the
// table. The result contains no colour escape codes, so it is safe under
// --no-color and in pipes.
func Sparkline(values []int64, width int) string {
	if len(values) == 0 || width <= 0 {
		return "—"
	}

	var max int64
	for _, v := range values {
		if v > max {
			max = v
		}
	}

	// Downsample by taking the maximum of each bucket. The last bucket absorbs
	// any remainder so every input value is represented.
	if len(values) > width {
		bucketed := make([]int64, 0, width)
		for i := 0; i < width; i++ {
			start := i * len(values) / width
			end := (i + 1) * len(values) / width
			if end <= start {
				end = start + 1
			}
			var bucketMax int64
			for _, v := range values[start:end] {
				if v > bucketMax {
					bucketMax = v
				}
			}
			bucketed = append(bucketed, bucketMax)
		}
		values = bucketed
	}

	var b strings.Builder
	b.Grow(len(values) * 3)
	for _, v := range values {
		idx := 0
		if max > 0 && v > 0 {
			// 1..len(blocks)-1 so a non-zero value is never the flat baseline.
			idx = 1 + int((v*(int64(len(sparkBlocks))-2))/max)
			if idx >= len(sparkBlocks) {
				idx = len(sparkBlocks) - 1
			}
		}
		b.WriteRune(sparkBlocks[idx])
	}
	return b.String()
}
