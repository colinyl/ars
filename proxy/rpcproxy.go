package proxy

import (
	"runtime/debug"
	"sync"

	"github.com/colinyl/ars/cluster"
	"github.com/colinyl/ars/rpc"
	"github.com/colinyl/lib4go/concurrent"
	"github.com/colinyl/lib4go/logger"
)

//ISnap 快照信息接口
type ISnap interface {
	GetSnap() string
}

//RPCClientProxy 处理JOB Consumer操作
type RPCClientProxy struct {
	tasks         concurrent.ConcurrentMap
	clusterClient cluster.IClusterClient
	client        *rpc.RPCClient
	Log           logger.ILogger
	snap          ISnap
	lock          sync.RWMutex
}

//NewRPCClientProxy 构建JNewRPCClientProxy处理对象
func NewRPCClientProxy(client cluster.IClusterClient, rpcClient *rpc.RPCClient, snap ISnap, loggerName string) *RPCClientProxy {
	job := &RPCClientProxy{}
	job.clusterClient = client
	job.client = rpcClient
	job.snap = snap
	job.tasks = concurrent.NewConcurrentMap()
	job.Log, _ = logger.Get(loggerName)
	return job
}

//GetTasks 获取当前已注册task列表
func (h *RPCClientProxy) GetTasks() map[string]cluster.TaskItem {
	data := make(map[string]cluster.TaskItem)
	service := h.tasks.GetAll()
	for i, v := range service {
		data[i] = v.(cluster.TaskItem)
	}
	return data
}

//OpenTask 启动新的任务
func (h *RPCClientProxy) OpenTask(task cluster.TaskItem) {
	h.tasks.Set(task.Name, task)
	h.Log.Info("::start service:", task.Name)
}

//CloseTask 关闭任务
func (h *RPCClientProxy) CloseTask(ti cluster.TaskItem) {
	h.Log.Info(" -> close service:", ti.Name)
	h.tasks.Delete(ti.Name)
}

//Request 执行Request请求
func (h *RPCClientProxy) Request(ti cluster.TaskItem, input string, session string) (r string, err error) {
	defer h.recover()
	r, er := h.client.Request(ti.Name, input, session)
	if er != nil {
		r = er.Error()
	}
	return
}

//Send 暂不支持
func (h *RPCClientProxy) Send(ti cluster.TaskItem, input string, data []byte) (string, error) {
	return h.client.Send(ti.Name, input, string(data))
}

//Get 暂不支持
func (h *RPCClientProxy) Get(ti cluster.TaskItem, input string) ([]byte, error) {
	data, err := h.client.Get(ti.Name, input)
	if err != nil {
		return nil, err
	}
	return []byte(data), nil
}
func (h *RPCClientProxy) getResult(result []string, er error) (r string, err error) {
	err = er
	if err != nil {
		return
	}
	if len(result) > 0 {
		r = result[0]
	}
	return
}

func (h *RPCClientProxy) recover() {
	if r := recover(); r != nil {
		h.Log.Fatal(r, string(debug.Stack()))
	}
}