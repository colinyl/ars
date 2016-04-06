/*
提供服务提供商功能
1. 下载服务提供商配置文件，并根据服务模式分组，检查当前是否是独占服务器，如果是检查当前服务是否正确，正确则退出，否则轮循每个配置
2. 检查当前机器IP是否与配置相符，不符则重复执行步骤2检查下一个配置，否则转到步骤3
3. 创建当前服务配置节点
4. 检查当前服务的数量是否超过配置数据，如果超过则删除当前节点
5. 检查当前配置是否是独占，如果是则返回状态，不再继续绑定服务，否则转到步骤2绑定下一服务
6. 标记当前服务器是独占还是共享，如果是共享则转到步骤7执行，否则转到步骤8执行
7. 监控所有独占服务变化，变化后，重新绑定当前服务，绑定成功后删除所有共享服务
8. 监控服务配置信息变化，变化后执行步骤1

*/

package cluster

import (
	"fmt"
	"log"
	"sync"

	"github.com/colinyl/lib4go/logger"
	"github.com/colinyl/lib4go/utility"
)

const (
	serviceRoot          = "@domain/sp"
	serviceConfig        = "@domain/configs/sp/config"
	serviceProviderRoot  = "@domain/sp/@serviceName/providers"
	serviceProviderPath  = "@domain/sp/@serviceName/providers/@ip@port"
	serviceProviderValue = `{"last":@last}`

	servicePublishPath    = "@domain/configs/sp/publish"
	serviceProviderConfig = "@domain/configs/sp/config"
)

type serviceGroup struct {
	service []string
	index   int
}
type servicesMap struct {
	data  map[string]*serviceGroup
	lk    sync.Mutex
	index int
}
type spService struct {
	Name   string
	IP     string
	Mode   string
	Script string
}
type spConfig struct {
	services map[string]*spService
}

type spServer struct {
	Path          string
	dataMap       *utility.DataMap
	Last          int64
	Log           *logger.Logger
	Port          string
	services      *spConfig
	lk            sync.Mutex
	mode          string
	serviceConfig string
}

var (
	eModeShared = "shared"
	eModeAlone  = "alone"
)
func NewServiceMap() *servicesMap {
	return &servicesMap{data: make(map[string]*serviceGroup)}
}
func (s *servicesMap) setData(data map[string][]string) {
	s.lk.Lock()
	defer s.lk.Unlock()
	s.data = make(map[string]*serviceGroup)
	for i, v := range data {
		if _, ok := s.data[i]; !ok {
			s.data[i] = &serviceGroup{}
		}
		for _, k := range v {
			s.data[i].service = append(s.data[i].service, k)
		}
	}
}
func (s *servicesMap) get(name string) (ip string) {
	s.lk.Lock()
	defer s.lk.Unlock()
	ip = ""
	group, ok := s.data[name]
	if !ok {
		return
	}
	ip = group.service[group.index%len(group.service)]
	group.index++
	return
}



func (d *spService) getUNIQ() string {
	return fmt.Sprintf("%s|%s|%s", d.Name, d.IP, d.Mode)
}

type ServiceProviderList map[string][]string

//Add  add a service to list
func (s ServiceProviderList) Add(serviceName string, server string) {
	if s[serviceName] == nil {
		s[serviceName] = []string{}
	}
	s[serviceName] = append(s[serviceName], server)
}

func NewSPServer() *spServer {
	var err error
	sp := &spServer{}
	sp.dataMap = utility.NewDataMap()
	sp.dataMap.Set("domain", zkClient.Domain)
	sp.dataMap.Set("ip", zkClient.LocalIP)
	sp.Log, err = logger.New("service provider", true)
	sp.services = &spConfig{}
	sp.services.services = make(map[string]*spService, 0)
	sp.serviceConfig = sp.dataMap.Translate(serviceConfig)
	if err != nil {
		log.Println(err)
	}
	return sp
}

