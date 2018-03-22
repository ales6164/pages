package pages


// Mustache encoder
func Encode(t string) string {
	return replaceAllGroupFunc(reTemplate, t, func(groups []string) string {
		tag, val := groups[1], groups[2]

		if val == "content" {
			return `<!--stache-content-->`
		}

		var stache string
		if len(tag) > 0 {
			stache = "{{" + tag + " " + val + "}}"
		}
		if len(val) > 0 {
			stache = "{{" + val + "}}"
		}

		return `<!--stache:` + stache + `-->`
	})
}

// Mustache decoder
func Decode(t string) string {
	return replaceAllGroupFunc(reDecode, t, func(groups []string) string {
		tag, val := groups[1], groups[2]
		var stache string
		if len(tag) > 0 {
			stache = "{{" + tag + " " + val + "}}"
		}
		if len(val) > 0 {
			stache = "{{" + val + "}}"
		}
		return stache
	})
}
