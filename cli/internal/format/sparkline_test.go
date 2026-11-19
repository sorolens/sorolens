package format_test

import (
	"testing"

	"github.com/sorolens/sorolens/cli/internal/format"
)

func TestSparklineRendersUnicodeBlocks(t *testing.T) {
	got := format.Sparkline([]int64{1, 4, 2, 8, 5, 0}, 12)
	if got != "▂▅▃█▅▁" {
		t.Errorf("got %q, want %q", got, "▂▅▃█▅▁")
	}
}

func TestSparklineEmptyAndZeroWidth(t *testing.T) {
	if got := format.Sparkline(nil, 12); got != "—" {
		t.Errorf("empty: got %q, want em dash", got)
	}
	if got := format.Sparkline([]int64{1, 2}, 0); got != "—" {
		t.Errorf("zero width: got %q, want em dash", got)
	}
}

func TestSparklineDownsamplesByBucketMaximum(t *testing.T) {
	got := format.Sparkline([]int64{0, 0, 9, 0, 0, 0, 0, 0}, 4)
	if got != "▁█▁▁" {
		t.Errorf("got %q, want the peak preserved in the second bucket", got)
	}
}

func TestSparklineAllZeroIsFlat(t *testing.T) {
	got := format.Sparkline([]int64{0, 0, 0}, 3)
	if got != "▁▁▁" {
		t.Errorf("got %q, want flat low blocks", got)
	}
}
