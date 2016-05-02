package cluster

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/colinyl/ars/config"
	"github.com/colinyl/ars/rpcservice"
	"github.com/colinyl/lib4go/logger"
	"github.com/colinyl/lib4go/utility"
	"github.com/colinyl/lib4go/webserver"
)

const (
	appServerConfig = "@domain/app/config/@ip"
	appServerPath   = "@domain/app/servers/@ip"
	jobConsumerPath = "@domain/job/servers/@jobName/job_"

	//jobConsumerValue = `{"ip":"@ip@jobPort","last":@now}`
)

type appSnap struct {
	Address string          `json:"address"`
	Last    int64           `json:"last"`
	Sys     *sysMonitorInfo `json:"sys"`
}

func (a appSnap) GetSnap() string {
	snap := a
	snap.Last = time.Now().Unix()
	snap.Sys, _ = GetSysMonitorInfo()
	buffer, _ := json.Marshal(&snap)
	return string(buffer)
}

type taskConfig struct {
	Trigger string `json:"trigger"`
	Script  string `json:"script"`
}
type taskRouteConfig struct {
	Path   string `json:"path"`
	Method string `json:"method"`
	Script string `json:"script"`
}

type serverConfig struct {
	ServerType string             `json:"type"`
	Routes     []*taskRouteConfig `json:"routes"`
}
type AppConfig struct {
	Status string        `json:"status"`
	Tasks  []*taskConfig `json:"tasks"`
	Jobs   []string      `json:"jobs"`
	Server *serverConfig `json:"server"`
}

type RCServerConfig struct {
	Domain  string
	Address string
	Server  string
}
type scriptHandler struct {
	data   *taskRouteConfig
	server *appServer
}

type appServer struct {
	dataMap           *utility.DataMap
	Last              int64
	Log               *logger.Logger
	zkClient          *zkClientObj
	appServerConfig   string
	rcServerRoot      string
	rcServerPool      *rpcservice.RPCServerPool
	scriptEngine      *scriptEngine
	rcServicesMap     *servicesMap
	jobServer         *rpcservice.RPCServer
	hasStartJobServer bool
	jobServerAdress   string
	appServerAddress  string
	lk                sync.Mutex
	jobNames          map[string]string
	apiServer         *webserver.WebServer
	apiServerAddress  string
	appRoutes         []*taskRouteConfig
	scriptHandlers    map[string]*scriptHandler
	snap           appSnap
}

func NewAPPServer() *appServer {
	app := &appServer{}
	app.Log, _ = logger.New("app server", true)
	return app
}
func (app *appServer) init() (err error) {
	app.zkClient = NewZKClient()
	app.dataMap = app.zkClient.dataMap.Copy()
	app.appServerConfig = app.dataMap.Translate(appServerConfig)
	app.rcServerRoot = app.dataMap.Translate(rcServerRoot)
	app.appServerAddress = app.dataMap.Translate(appServerPath)
	app.rcServerPool = rpcservice.NewRPCServerPool()
	app.scriptEngine = NewScriptEngine(app)
	app.rcServicesMap = NewServiceMap()
	app.jobNames = make(map[string]string)
	app.scriptHandlers = make(map[string]*scriptHandler)
	app.snap = appSnap{config.Get().IP, 0, nil}
	return
}
func (r *appServer) Start() (err error) {
	if err = r.init(); err != nil {
		return
	}
	r.WatchRCServerChange(func(config []*RCServerConfig, err error) {
		r.BindRCServer(config, err)
	})

	r.WatchConfigChange(func(config *AppConfig, err error) error {
		r.BindTask(config, err)
		return nil
	})

	return nil
}

func (r *appServer) Stop() error {
	defer func() {
		recover()
	}()

	r.zkClient.ZkCli.Close()
	if r.jobServer != nil {
		r.jobServer.Stop()
	}
	r.Log.Info("::app server closed")
	return nil
}
