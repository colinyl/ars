package main

import "github.com/colinyl/ars/cluster"

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
					rc.bindCrossServices(domain, items)
					rc.PublishNow()
				})
			}(domain)
		}
	}
}
func (rc *RCServer) getDomainIPs(items []*cluster.RCServerItem) []string {
	var ips = []string{}
	for _, v := range items {
		ips = append(ips, v.Address)
	}
	return ips
}
func (rc *RCServer) bindCrossServices(domain string, items []*cluster.RCServerItem) {
	ips := rc.getDomainIPs(items)
	allServices := rc.crossService.Get(domain).(cluster.ServiceProviderList)
	for name := range allServices {
		allServices[name] = ips
	}
}
