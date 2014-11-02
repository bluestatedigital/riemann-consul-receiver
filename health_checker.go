package main

import (
    "time"
    log "github.com/Sirupsen/logrus"

    "github.com/armon/consul-api"
)

type HealthCheck struct {
    // *consulapi.HealthCheck // <- why isn't that working?
    Node        string
    CheckID     string
    Name        string
    Status      string
    Notes       string
    Output      string
    ServiceID   string
    ServiceName string
    Tags        []string
}

type nodeServiceKey struct {
    Node      string
    ServiceID string
}

type HealthChecker struct {
    health         ConsulHealth
    catalog        ConsulCatalog
    updateInterval time.Duration
}

func NewHealthChecker(health ConsulHealth, catalog ConsulCatalog, updateInterval time.Duration) *HealthChecker {
    return &HealthChecker{
        health: health,
        catalog: catalog,
        updateInterval: updateInterval,
    }
}

func (self *HealthChecker) WatchHealthResults(done <-chan interface{}) <-chan []HealthCheck {
    resultsChan := make(chan []HealthCheck)
    
    waitIdx := uint64(0)
    keepWatching := true
    
    go func() {
        for keepWatching {
            log.Debugf("retrieving health results; WaitIndex=%d", waitIdx)
            
            // maintain map of node/serviceId to CatalogService details.  reset each
            // time we refresh the health checks.  this is to ensure the service
            // tags coincide with the services we're reporting on.  this map's
            // messy, but it ensures that we only retrieve service details once per
            // service.
            serviceDetails := make(map[string]map[nodeServiceKey]*consulapi.CatalogService)

            healthChecks, queryMeta, err := self.health.State("any", &consulapi.QueryOptions{
                WaitIndex: waitIdx,
                WaitTime:  self.updateInterval,
            })
            
            if err != nil {
                log.Errorf("error retrieving health results: %v", err)
                break
            }
            
            // LastIndex used for blocking query
            waitIdx = queryMeta.LastIndex
            
            log.Debug("handling health check results")
            
            var results []HealthCheck
            for _, hc := range healthChecks {
                result := HealthCheck{
                    Node:        hc.Node,
                    CheckID:     hc.CheckID,
                    Name:        hc.Name,
                    Status:      hc.Status,
                    Notes:       hc.Notes,
                    Output:      hc.Output,
                    ServiceID:   hc.ServiceID,
                    ServiceName: hc.ServiceName,
                }
                
                if hc.ServiceID != "" {
                    if _, exists := serviceDetails[hc.ServiceName]; ! exists {
                        // retrieve the service details; don't already have them
                        svcDetails, _, err := self.catalog.Service(hc.ServiceName, "", nil)
                        
                        if err != nil {
                            // break out of the HealthCheck iteration loop if an
                            // error occurs retrieving the services
                            log.Errorf("error retrieving services: %v", err)
                            keepWatching = false
                            break
                        }
                        
                        // store service details in map
                        serviceDetails[hc.ServiceName] = make(map[nodeServiceKey]*consulapi.CatalogService)
                        for _, svcDetail := range svcDetails {
                            serviceDetails[hc.ServiceName][nodeServiceKey{svcDetail.Node, svcDetail.ServiceID}] = svcDetail
                        }
                    }
                    
                    // set the HealthCheck's Tags to the service's tags
                    if _, exists := serviceDetails[hc.ServiceName]; exists {
                        if svcDetail, exists := serviceDetails[hc.ServiceName][nodeServiceKey{hc.Node, hc.ServiceID}]; exists {
                            result.Tags = svcDetail.ServiceTags
                        } else {
                            log.Errorf("no service details found for %s, %s", hc.Node, hc.ServiceID)
                        }
                    } else {
                        log.Errorf("service %s not found for %s, %s", hc.ServiceName, hc.Node, hc.ServiceID)
                    }
                }
                
                results = append(results, result)
            }
            
            // keepWatching might have been set to false if an error occurred
            // retrieving the services
            if keepWatching {
                log.Debug("sending health results")
                select {
                    case resultsChan <- results:
                        // successfully sent results
                    
                    case <-done:
                        // channel's closed when we've been told to stop
                        keepWatching = false
                }
            }
        }
        
        log.Info("health results watch stopped")
        close(resultsChan)
    }()
    
    return resultsChan
}
