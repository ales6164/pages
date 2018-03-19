package pages

import (
	"regexp"
	"strings"
)

type Template struct {
	content string

	i                   int
	opened              []Func
	predefinedFuncCalls []string
}

var (
	reTemplate = regexp.MustCompile(`\{\{\s*(?P<tag>\>|\#|\/|\^|\!|)\s*(?P<var>[a-zA-Z\-]+)\s*\}\}`)
)

//"customComponents.define(" + f.name + ",($,$$$)=>{let $$=$;return`"
//"`}" + predefFuns + ");"
func ConvertMustache(name string, html string) string {
	t := new(Template)
	return "customComponents.setTemplate('" + name + "',function($){let $$=$;return" + t.Compile(html) + "});"
}

// compile file
func (t *Template) Compile(html string) string {
	t.content = html

	// escape single quotes
	t.content = regexp.MustCompile("\x60").ReplaceAllString(t.content, "\\\x60")

	// compile into JS template literal
	t.content = replaceAllGroupFunc(reTemplate, t.content, func(groups []string) string {
		//fmt.Print(groups[1])
		return t.replace(groups[1], groups[2])
	})

	return "\x60" + t.content + "\x60"
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
		}
		return "${$$." + matchedVar + "}"
	}
	if strings.HasPrefix(matchedVar, "$") {
		return matchedVar
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

func (f *funcWith) start() string {
	if f.reversed {
		return "${!" + f.matchedVar + "||" + f.matchedVar + ".constructor===Array?(($$)=>{$$=$$.constructor===Array?$$:[$$];return $$.reverse().map(($$, _i)=>{return`"
	}
	return "${" + f.matchedVar + "?(($$)=>{$$=$$.constructor===Array?$$:[$$];return $$.map(($$, _i)=>{return`"
}

func (f *funcWith) end() string {
	return "`}).join()})(" + f.matchedVar + "):``}"
}
