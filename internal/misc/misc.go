package misc

func StringOrDefault(v, d string) string {
	if len(v) == 0 {
		return d
	}
	return v
}
