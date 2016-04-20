package cluster

import (
	"strings"

	"github.com/colinyl/ars/scheduler"
)

func (a *appServer) BindTask(config *AppConfig, err error) error {
	a.resetSnap(a.appServerAddress, config)
	scheduler.Stop()
	for _, v := range config.Tasks {
		scheduler.AddTask(v.Trigger, scheduler.NewTask(v.Script, func(name string) {
			a.Log.Infof("start:%s", name)
			rtvalues, err := a.scriptEngine.pool.Call(name)
			if err != nil {
				a.Log.Error(err)
			} else {
				a.Log.Infof("result:%d,%s",len(rtvalues),strings.Join(rtvalues, ","))
			}
		}))
	}
	a.Log.Infof("task:%d,job:%d,script:%d", len(config.Tasks), len(config.Jobs), len(config.ScriptServer))
	if len(config.Jobs) > 0 {
		a.StartJobConsumer(config.Jobs)
	} else {
		a.StopJobServer()
	}
	if len(config.Tasks) > 0 {
		scheduler.Start()
	} else {
		scheduler.Stop()
	}
	if len(config.ScriptServer) > 0 {
		a.scriptServer = config.ScriptServer
		a.StopAPIServer()
		a.StartAPIServer()
	} else {
		a.StopAPIServer()
	}

	return nil
}
