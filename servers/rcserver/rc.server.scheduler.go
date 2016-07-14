package main

import (
	"github.com/colinyl/ars/cluster"
	"github.com/colinyl/ars/rpcproxy"
	"github.com/colinyl/ars/rpcservice"
	"github.com/colinyl/lib4go/scheduler"
	"github.com/colinyl/lib4go/utility"
)

//BindJobScheduler 绑定RC服务器的JOB任务
func (rc *RCServer) BindJobScheduler(jobs map[string]cluster.JobItem, err error) {
	if err != nil {
		rc.Log.Error(err)
		return
	}

	scheduler.Stop()

	if len(jobs) == 0 {
		return
	}
	var jobCount int
	for _, v := range jobs {
		if v.Concurrency <= 0 || !v.Enable {
			continue
		}
		jobCount++
		scheduler.AddTask(v.Trigger, scheduler.NewTask(v, func(v interface{}) {
			task := v.(cluster.JobItem)
			consumers := rc.clusterClient.GetJobConsumers(task.Name)
			total := jobs[task.Name].Concurrency
			index := 0
			for i := 0; i < len(consumers); i++ {
				client := rpcservice.NewRPCClient(consumers[i], rc.loggerName)
				if client.Open() != nil {
					rc.Log.Infof("open rpc server(%s) error ", consumers[i])
					continue
				}
				result, err := client.Request(task.Name, "{}", utility.GetSessionID())
				client.Close()
				if err != nil {
					rc.Log.Error(err)
					continue
				}
				if !rpcproxy.ResultIsSuccess(result) {
					rc.Log.Infof(" ->call job(%s - %v) failed %s", task.Name, consumers[i], result)
					continue
				} else {
					rc.Log.Infof(" ->call job(%s - %s) success", task.Name, consumers[i])
				}
				index++
				if index >= total {
					continue
				}
			}
			if index < total {
				rc.Log.Infof("job(%s) has executed (%d/%d),consumers:%d", task.Name, index, total, len(consumers))
			}

		}))
	}
	if jobCount > 0 {
		rc.Log.Infof("job config has changed:%d", jobCount)
		scheduler.Start()
	}

}
