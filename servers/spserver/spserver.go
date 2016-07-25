package main

import (
	"fmt"
	"sync"

	"github.com/colinyl/ars/cluster"
	"github.com/colinyl/ars/mq"
	"github.com/colinyl/ars/proxy"
	"github.com/colinyl/ars/rpc"
	"github.com/colinyl/ars/script"
	"github.com/colinyl/ars/server"
	"github.com/colinyl/ars/servers/config"
	"github.com/colinyl/lib4go/concurrent"
	"github.com/colinyl/lib4go/logger"
)

var (
	eModeShared = "share"
	eModeAlone  = "alone"
)

//SPServer SPServer
type SPServer struct {
	Log            logger.ILogger
	lk             sync.Mutex
	domain         string
	mode           string
	serviceConfig  string
	mqService      *mq.MQConsumerService
	rpcClient      *rpc.RPCClient
	rpcServer      *server.RPCServer  //RPC 服务器
	rpcScriptProxy *proxy.ScriptProxy //RPC Server 脚本处理程序
	clusterClient  cluster.IClusterClient
	scriptPool     *script.ScriptPool //脚本引擎池
	dbPool         concurrent.ConcurrentMap
	snap           SPSnap
	loggerName     string
}

//NewSPServer 创建SP server服务器
func NewSPServer() *SPServer {
	sp := &SPServer{loggerName: "sp.server"}
	sp.Log, _ = logger.Get(sp.loggerName)
	return sp
}

//init 初始化服务器
func (sp *SPServer) init() (err error) {
	defer sp.recover()
	cfg, err := config.Get()
	if err != nil {
		return
	}
	sp.Log.Infof(" -> 初始化 %s...", cfg.Domain)

	sp.clusterClient, err = cluster.GetClusterClient(cfg.Domain, cfg.IP, sp.loggerName, cfg.ZKServers...)
	if err != nil {
		return
	}
	sp.domain = cfg.Domain
	sp.snap = SPSnap{ip: cfg.IP}
	sp.rpcClient = rpc.NewRPCClient(sp.clusterClient, sp.loggerName)
	sp.scriptPool, err = script.NewScriptPool(sp.clusterClient, sp.rpcClient, sp.GetScriptBinder(), sp.loggerName)
	if err != nil {
		return
	}
	sp.rpcScriptProxy = proxy.NewScriptProxy(sp.clusterClient, sp.scriptPool, sp.loggerName)
	sp.rpcScriptProxy.OnOpenTask = sp.OnSPServiceCreate
	sp.rpcScriptProxy.OnCloseTask = sp.OnSPServiceClose
	sp.rpcServer = server.NewRPCServer(sp.rpcScriptProxy, sp.loggerName)
	sp.mqService, err = mq.NewMQConsumerService(sp.clusterClient, mq.NewMQScriptHandler(sp.scriptPool, sp.loggerName), sp.loggerName)
	sp.dbPool = concurrent.NewConcurrentMap()
	return
}

//Start 启动SP Server服务器
func (sp *SPServer) Start() (err error) {
	defer sp.recover()
	sp.Log.Info(" -> 启动SP Server...")
	if err = sp.init(); err != nil {
		sp.Log.Error(err)
		return
	}
	if !sp.clusterClient.WatchConnected() {
		return
	}
	sp.rpcServer.Start()
	sp.snap.Address = fmt.Sprint(sp.snap.ip, sp.rpcServer.Address)
	sp.clusterClient.WatchSPTaskChange(func() {
		sp.rebindService()
	})
	sp.clusterClient.WatchRCServerChange(func(config []*cluster.RCServerItem, err error) {
		sp.BindRCServer(config, err)
	})

	go sp.StartRefreshSnap()
	return nil
}

//Stop 停止SP Server服务器
func (sp *SPServer) Stop() error {
	defer sp.recover()
	sp.Log.Info(" -> 退出SP Server...")
	sp.clusterClient.Close()
	sp.rpcClient.Close()
	sp.rpcServer.Stop()
	sp.Log.Info("::sp server closed")
	return nil
}
