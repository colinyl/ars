package script

import (
	"github.com/arsgo/lib4go/influxdb"
	"github.com/arsgo/lib4go/script"
	lua "github.com/yuin/gopher-lua"
)

//influxdb操作类，用于lua脚本直接调用
//local influx,err=influxdb.new("influx")
//if err~=nil then
//	 print(err)
//end
//influx:save("{"id":1,"name":"colin"}")
func (s *ScriptPool) getinfluxTypeBinder() script.LuaTypesBinder {
	return script.LuaTypesBinder{
		Name: "influxdb",
		NewFunc: map[string]lua.LGFunction{
			"new": s.typeInfluxType,
		},
		Methods: map[string]lua.LGFunction{
			"save": typeInfluxDBSave,
		},
	}
}

// Constructor
func (s *ScriptPool) typeInfluxType(L *lua.LState) int {
	var err error
	ud := L.NewUserData()
	name := L.CheckString(1)
	ud.Value, err = s.NewInfluxDB(name)
	if err != nil {
		return pushValues(L, "", err)
	}
	L.SetMetatable(ud, L.GetTypeMetatable("influxdb"))
	L.Push(ud)
	return 1
}

// Checks whether the first lua argument is a *LUserData with *Person and returns this *Person.
func checkInfluxDBType(L *lua.LState) *influxdb.InfluxDB {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*influxdb.InfluxDB); ok {
		return v
	}
	L.RaiseError("bad argument  (influxdb.InfluxDB expected, got %s)", ud.Type().String())
	return nil
}
func typeInfluxDBSave(L *lua.LState) int {
	p := checkInfluxDBType(L)
	row := L.CheckString(2)
	a := p.Save(row)
	return pushValues(L, a)
}
