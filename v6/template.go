package pages

import (
	"bytes"
	"regexp"
	"strings"
)

type Template struct {
	dbgCtx  *bytes.Buffer
	content string

	i                   int
	opened              []Func
	predefinedFuncCalls []string
}

var (
	reTemplate = regexp.MustCompile(`\{\{\s*(\>|\#|\/|\^|\!|)\s*([a-zA-Z\-\.\_\$]+)\s*\}\}`)
	reDecode   = regexp.MustCompile(`(?:<|&lt;)!--stache:(\>|\#|\/|\^|\!|)\s*([a-zA-Z\-\.\_\$]+)--(?:>|&gt;)`)
)

//"customComponents.define(" + f.name + ",($,$$$)=>{let $$=$;return`"
//"`}" + predefFuns + ");"
func ConvertMustache(html string, decode bool) string {
	t := new(Template)
	t.dbgCtx = new(bytes.Buffer)
	return t.Compile(html, decode)
}

func DebugConvertMustache(w *bytes.Buffer, html string, decode bool) string {
	t := new(Template)
	t.dbgCtx = w
	return t.Compile(html, decode)
}

// compile file
func (t *Template) Compile(html string, decode bool) string {
	t.content = html

	// escape single quotes
	t.content = regexp.MustCompile("\x60").ReplaceAllString(t.content, "\\\x60")

	t.dbgCtx.WriteString("compiling")

	//var str []string
	// compile into JS template literal
	var reg *regexp.Regexp
	if decode {
		reg = reDecode
	} else {
		reg = reTemplate
	}

	t.content = replaceAllGroupFunc(reg, t.content, func(groups []string) string {
		//fmt.Print(groups[1])
		//str = append(str, groups[1])
		//log.Infof(t.dbgCtx, "tag %s var %s", groups[1], groups[2])

		t.dbgCtx.WriteString(groups[1])
		t.dbgCtx.WriteString(groups[2] + "\n")

		return t.replace(groups[1], groups[2])
	})

	//var str []string

	/*t.content = reTemplate.ReplaceAllStringFunc(t.content, func(s string) string {
		s = strings.TrimPrefix(s, "{{")
		s = strings.TrimSuffix(s, "}}")
		s = strings.TrimSpace(s)

		var matchedTag string
		if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "^") || strings.HasPrefix(s, "/") {
			matchedTag = s[:1]
			s = s[1:]
			s = strings.TrimSpace(s)
		}

		//str = append(str, s)
		return t.replace(matchedTag, s)
	})*/

	return t.content
}

func (t *Template) replace(matchedTag, matchedVar string) (rendered string) {
	switch matchedTag {
	case "#":
		t.putFunc(FuncWith(evalMatchedVar(matchedVar, false), false))
		rendered = t.opened[t.i].start()
	case "^":
		t.putFunc(FuncWith(evalMatchedVar(matchedVar, false), true))
		rendered = t.opened[t.i].start()
	case "/":
		rendered = t.endFunc()
	default:
		rendered = evalMatchedVar(matchedVar, true)
	}
	return rendered
}

func (t *Template) putFunc(f Func) {
	t.opened = append(t.opened, f)
	t.i = len(t.opened) - 1
}

func (t *Template) endFunc() string {
	end := t.opened[t.i].end()
	t.opened = t.opened[:len(t.opened)-1]
	t.i = len(t.opened) - 1
	return end
}

func evalMatchedVar(matchedVar string, encapsulate bool) string {
	if encapsulate {
		if strings.HasPrefix(matchedVar, "$") {
			return "${" + matchedVar + "}"
		} else if matchedVar == "." {
			return "${$$}"
		}
		return "${$$." + matchedVar + "}"
	}
	if strings.HasPrefix(matchedVar, "$") {
		return matchedVar
	} else if matchedVar == "." {
		return "$$"
	}
	return "$$." + matchedVar
}

type Func interface {
	start() string
	end() string
}

/* WITH */

type funcWith struct {
	matchedVar string
	reversed   bool
}

func FuncWith(matchedVar string, reversed bool) *funcWith {
	return &funcWith{
		matchedVar: matchedVar,
		reversed:   reversed,
	}
}

/*func (f *funcWith) start() string {
	if f.reversed {
		return "${!" + f.matchedVar + "||" + f.matchedVar + ".constructor===Array?(($$)=>{$$=$$&&$$.constructor===Array?$$:[$$];return $$.reverse().map(($$, _i)=>{return`"
	}
	return "${" + f.matchedVar + "?(($$)=>{$$=$$.constructor===Array?$$:[$$];return $$.map(($$, _i)=>{return`"
}

func (f *funcWith) end() string {
	return "`}).join()})(" + f.matchedVar + "):``}"
}*/

func (f *funcWith) start() string {
	if f.reversed {
		return "${rearr(" + f.matchedVar + ").map(($$,_i)=>{return html\x60"
	}
	return "${arr(" + f.matchedVar + ").map(($$,_i)=>{return html\x60"
}

func (f *funcWith) end() string {
	return "\x60})}"
}
