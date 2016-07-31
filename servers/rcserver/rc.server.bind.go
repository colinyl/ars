package main

import (
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/colinyl/ars/cluster"
)

//BindRCServer 绑定服务
func (rc *RCServer) BindRCServer() (err error) {
	rc.Log.Info("------bind rcserver")
	rc.snap.Address = fmt.Sprint(rc.snap.ip, rc.rcRPCServer.Address)
	rc.snap.Path, err = rc.clusterClient.CreateRCServer(rc.snap.GetSnap())
	if err != nil {
		return
	}
	rc.clusterClient.UpdateSnap(rc.snap.Path, rc.snap.GetSnap())
	rc.clusterClient.WatchRCServerChange(func(items []*cluster.RCServerItem, err error) {
		isMaster := rc.IsMasterServer(items)
		if isMaster && !rc.IsMaster {
			rc.IsMaster = true
			rc.snap.Server = SERVER_MASTER
			rc.Log.Info("::current server is ", rc.snap.Server)

			go rc.clusterClient.WatchJobConfigChange(func(config map[string]cluster.JobItem, err error) {
				rc.BindJobScheduler(config, err)
			})
			go rc.clusterClient.WatchServiceProviderChange(func(lst cluster.ServiceProviderList, err error) {
				//重新发布服务
				rc.Log.Info(" |-> rpc service provider changed")
				rc.currentServices.Set("*", lst)
				rc.PublishNow()
				rc.startSync.Done("INIT.SERVER")
			})
			go rc.clusterClient.WatchRCTaskChange(func(task cluster.RCServerTask, err error) {
				if err != nil {
					rc.Log.Error(err)
					return
				}
				rc.spRPCClient.SetPoolSize(task.RPCPoolSetting.MinSize, task.RPCPoolSetting.MaxSize)
				rc.BindCrossAccess(task)
			})
		} else if !isMaster {
			rc.IsMaster = false
			rc.snap.Server = SERVER_SLAVE
			rc.Log.Info("::current server is ", rc.snap.Server)
			go rc.clusterClient.WatchRCTaskChange(func(task cluster.RCServerTask, err error) {
				rc.spRPCClient.SetPoolSize(task.RPCPoolSetting.MinSize, task.RPCPoolSetting.MaxSize)
				rc.startSync.Done("INIT.SERVER")
			})
		}
	})
	rc.startSync.WaitAndAdd(1)
	rc.clusterClient.WatchRPCServiceChange(func(services map[string][]string, err error) {
		defer rc.startSync.Done("INIT.SRV.CNG")
		rc.BindSPServers(services, err)
	})
	return
}

//startMonitor 启动监控服务
func (rc *RCServer) startMonitor() {

	/*	go func() {
		START:
			if rc.clusterClient.WaitForDisconnected() {
				rc.Log.Info("连接已断开")
				r := make(cluster.ServiceProviderList)
				rc.BindSPServers(r, nil)
				goto START
			}
		}()*/

	go func() {
		tk := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-tk.C:
				minServices := rc.crossDomain.GetLength() + 1
				currentServices := rc.spRPCClient.GetServiceCount()
				if currentServices < minServices {
					rc.Log.Info(" -> service len cant less than ", minServices, currentServices)
					rc.BindSPServers(rc.clusterClient.GetRPCService())
				}
			}
		}
	}()

START:
	if rc.clusterClient.WaitForConnected() {
		if rc.IsMaster {
			rc.Log.Info(" |-> 已重新连接，重新发布服务")
			rc.PublishAll()
		}
		goto START
	}
}

//PublishAll 发布所有服务
func (rc *RCServer) PublishAll() {
	currentServices, err := rc.clusterClient.GetServiceProviders()
	if err != nil {
		rc.Log.Error(err)
		return
	}
	rc.currentServices.Set("*", currentServices)
	crossClusters := rc.crossDomain.GetAll()
	for domain, clt := range crossClusters {
		client := clt.(cluster.IClusterClient)
		crossService, err := client.GetServiceProviders()
		if err != nil {
			rc.Log.Error(err)
			continue
		}
		rc.crossService.Set(domain, crossService)
	}
	rc.PublishNow()
}

//BindSPServers 绑定service provider servers
func (rc *RCServer) BindSPServers(services map[string][]string, err error) {
	if err != nil {
		return
	}
	rc.Log.Info(" |-> rpc services changed")
	ip := rc.spRPCClient.ResetRPCServer(services)
	tasks, er := rc.clusterClient.FilterRPCService(services)
	if er != nil {
		rc.Log.Error(er)
		return
	}
	if rc.rcRPCServer.UpdateTasks(tasks) > 0 {
		rc.Log.Info(" |-> local services has changed:", len(tasks), ip)
	}
}

//PublishNow 立即发布服务
func (rc *RCServer) PublishNow() {
	defer func() {
		if r := recover(); r != nil {
			rc.Log.Fatal(r, string(debug.Stack()))
		}
	}()
	//立即发布服务
	services := rc.MergeService()
	rc.Log.Infof("->publish services:%d", len(services))
	err := rc.clusterClient.PublishRPCServices(services)
	if err != nil {
		rc.Log.Error(err)
	}
}

//BindCrossAccess 绑定垮域访问服务
func (rc *RCServer) BindCrossAccess(task cluster.RCServerTask) (err error) {
	//if !rc.IsMaster {
	//	return
	//}
	rc.ResetCrossDomainServices(task)
	rc.WatchCrossDomain(task)
	return
}

//ResetCrossDomainServices 重置跨域服务
func (rc *RCServer) ResetCrossDomainServices(task cluster.RCServerTask) {
	//添加、关闭、更新服务
	allServices := rc.crossService.GetAll()
	//添加不存在的域和服务
	for domain, item := range task.CrossDomainAccess {
		if _, ok := allServices[domain]; ok {
			continue
		}
		crossData := item.GetServicesMap()
		rc.crossService.Set(domain, crossData) //添加不存在的域服务
	}
	//删除，更新服务
	for domain, svs := range allServices {
		if _, ok := task.CrossDomainAccess[domain]; !ok {
			rc.crossService.Delete(domain) //不存在域,则删除
			continue
		}
		//检查本地服务是否与远程服务一致
		currentServices := svs.(cluster.ServiceProviderList)
		remoteServices := task.CrossDomainAccess[domain].GetServicesMap()
		//删除更新服务
		for name := range currentServices {
			if _, ok := remoteServices[name]; !ok {
				delete(currentServices, name)
			} else {
				currentServices[name] = task.CrossDomainAccess[domain].Servers
			}
		}
		//添加服务
		for name := range remoteServices {
			if _, ok := currentServices[name]; !ok {
				currentServices[name] = task.CrossDomainAccess[domain].Servers
			}
		}

	}
}

//WatchCrossDomain 监控外部域
func (rc *RCServer) WatchCrossDomain(task cluster.RCServerTask) {
	if len(task.CrossDomainAccess) == 0 {
		return
	}

	//关闭域
	currentCluster := rc.crossDomain.GetAll()
	for domain, clt := range currentCluster {
		if _, ok := task.CrossDomainAccess[domain]; !ok {
			client := clt.(cluster.IClusterClient)
			client.Close()
			rc.crossDomain.Delete(domain)
		}
	}

	//监控域
	for domain, v := range task.CrossDomainAccess {
		//为cluster类型时,添加监控
		if rc.crossDomain.Get(domain) == nil {
			clusterClient, err := cluster.GetClusterClient(domain, rc.snap.ip, rc.loggerName, v.Servers...)
			if err != nil {
				rc.Log.Error(err)
				continue
			}

			//将服务添加到服务列表
			rc.crossDomain.Set(domain, clusterClient)
			currentServices := make(cluster.ServiceProviderList)
			for _, svs := range v.Services {
				currentServices[svs] = v.Servers
			}
			rc.crossService.Set(domain, currentServices)

			//监控外部RC服务器变化,变化后更新本地服务
			go func(domain string) {
				defer rc.recover()
				rc.Log.Infof("::watch cross domain [%s] rc server change", domain)
				clusterClient.WatchRCServerChange(func(items []*cluster.RCServerItem, err error) {
					rc.Log.Infof("::cross domain [%s] rc server changed", domain)
					var ips = []string{}
					for _, v := range items {
						ips = append(ips, v.Address)
					}
					allServices := rc.crossService.Get(domain).(cluster.ServiceProviderList)
					for name := range allServices {
						allServices[name] = ips
					}
					rc.PublishNow()

				})
			}(domain)
		}
	}
}

//IsMasterServer 检查当前RC Server是否是Master
func (rc *RCServer) IsMasterServer(items []*cluster.RCServerItem) bool {
	var servers []string
	for _, v := range items {
		servers = append(servers, v.Path)
	}
	sort.Sort(sort.StringSlice(servers))
	return len(servers) == 0 || strings.EqualFold(rc.snap.Path, servers[0])
}

//MergeService 合并所有服务
func (rc *RCServer) MergeService() (lst cluster.ServiceProviderList) {
	lst = make(map[string][]string)
	services := rc.currentServices.Get("*")
	if services != nil {
		lst = services.(cluster.ServiceProviderList)
	}
	crossServices := rc.crossService.GetAll()
	for domain, svs := range crossServices {
		service := svs.(cluster.ServiceProviderList)
		for i, v := range service {
			if len(v) > 0 {
				lst[i+"@"+domain] = v
			}

		}
	}
	return lst
}
