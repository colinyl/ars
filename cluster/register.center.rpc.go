package cluster

import (
	"fmt"
	"strings"
	"time"

	"github.com/colinyl/ars/rpcservice"
	"github.com/colinyl/lib4go/logger"
)

type rcServerRPCHandler struct {
	server *rcServer
	Log    *logger.Logger
}

func (r *rcServerRPCHandler) Request(name string, input string) (result string, err error) {

	if strings.EqualFold(name, "test_request") {
		r.Log.Info("recv test_request")
		return "success", nil
	}
	r.Log.Infof("recv request:%s", name)
	group := r.server.spServicesMap.Next(name)
	//  return group,nil
	result, err = r.server.spServerPool.Request(group, name, input)
	if err != nil {
		result = fmt.Sprintf("%s(rc server)", err.Error())
		err = nil
	}
	return

}
func (r *rcServerRPCHandler) Send(name string, input string, data []byte) (string, error) {

	r.Log.Infof("recv request:%s", name)
	group := r.server.spServicesMap.Next(name)
	return r.server.spServerPool.Send(group, name, input, data)
}
func (r *rcServerRPCHandler) Get(name string, input string) ([]byte, error) {
	r.Log.Infof("recv request:%s", name)
	group := r.server.spServicesMap.Next(name)
	return r.server.spServerPool.Get(group, name, input)
}

func (d *rcServer) StartRPCServer() {
	port := rpcservice.GetLocalRandomAddress()
	d.snap.Address = fmt.Sprintf("%s%s", d.zkClient.LocalIP, port)
	d.rpcServer = rpcservice.NewRPCServer(port, &rcServerRPCHandler{server: d, Log: d.Log})
	d.rpcServer.Serve()
	time.Sleep(time.Second * 2)
	d.resetRCSnap()
}

func (r *rcServer) BindSPServer(services map[string][]string) {
	r.spServicesMap.setData(services)
	r.spServerPool.Register(getServers(services))
}

func getServers(services map[string][]string) map[string]string {
	servers := make(map[string]string)
	for _, v := range services {
		for _, value := range v {
			if _, ok := servers[value]; !ok {
				servers[value] = value
			}
		}
	}
	return servers
}
