package script

import (
	"github.com/arsgo/lib4go/encoding"
	"github.com/yuin/gopher-lua"
)

func (s *ScriptPool) moduleEncodingConvert(ls *lua.LState) int {
	input := ls.CheckString(1)
	chaset := ls.CheckString(2)
	result := encoding.Convert([]byte(input), chaset)
	return pushValues(ls, result)
}
func (s *ScriptPool) moduleUnicodeEncode(ls *lua.LState) int {
	input := ls.CheckString(1)
	result := encoding.UnicodeEncode(input)
	return pushValues(ls, result)
}
func (s *ScriptPool) moduleUnicodeDecode(ls *lua.LState) int {
	input := ls.CheckString(1)
	result := encoding.UnicodeDecode(input)
	return pushValues(ls, result)
}

func (s *ScriptPool) moduleURLEncode(ls *lua.LState) int {
	input := ls.CheckString(1)
	result := encoding.URLEncode(input)
	return pushValues(ls, result)
}
func (s *ScriptPool) moduleURLDecode(ls *lua.LState) int {
	input := ls.CheckString(1)
	result, err := encoding.URLDecode(input)
	return pushValues(ls, result, err)
}
func (s *ScriptPool) moduleHTMLEncode(ls *lua.LState) int {
	input := ls.CheckString(1)
	result := encoding.HTMLEncode(input)
	return pushValues(ls, result)
}
func (s *ScriptPool) moduleHTMLDecode(ls *lua.LState) int {
	input := ls.CheckString(1)
	result := encoding.HTMLDecode(input)
	return pushValues(ls, result)
}
