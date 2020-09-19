package pages

// Mustache encoder
func Encode(t string) string {
	return replaceAllGroupFunc(reTemplate, t, func(groups []string) string {
		tag, val := groups[1], groups[2]

		if val == "content" {
			return `<!--stache-content-->`
		}

		return `<!--stache:` + tag + val + `-->`
	})
}

// Mustache decoder
func Decode(t string) string {
	return replaceAllGroupFunc(reDecode, t, func(groups []string) string {
		tag, val := groups[1], groups[2]

		return "{{" + tag + val + "}}"
	})
}
