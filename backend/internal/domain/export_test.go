package domain

// Sample là một ca của bộ 100 ca chuẩn hóa (0.1 mục 19), mở cho test ở package domain_test.
type Sample struct {
	N          int
	Input      string
	GroupName  string
	DetailName string
	Qty        float64
	Unit       Unit
}

// Samples trả bộ 100 ca để test đối chiếu với taxonomy 0.2 (domain_test được import taxonomy).
func Samples() []Sample {
	out := make([]Sample, len(samples))
	for i, c := range samples {
		out[i] = Sample{c.n, c.input, c.groupName, c.detailName, c.qty, c.unit}
	}
	return out
}
