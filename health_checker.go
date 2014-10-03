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

func (self *HealthChecker) WatchHealthResults(resultsChan chan []HealthCheck) {
    waitIdx := uint64(0)
    keepWatching := true
    
    for keepWatching {
        log.Debugf("retrieving health results; WaitIndex=%d", waitIdx)
        
        // maintain map of node/serviceId to CatalogService details.  reset each
        // time we refresh the health checks.  this is to ensure the service
        // tags coincide with the services we're reporting on.
        serviceDetails := make(map[nodeServiceKey]*consulapi.CatalogService)

        healthChecks, queryMeta, err := self.health.State("any", &consulapi.QueryOptions{
            WaitIndex: waitIdx,
            WaitTime:  self.updateInterval,
        })
        
        if err != nil {
            log.Errorf("error retrieving health results: %v", err)
            break
        }
        
        waitIdx = queryMeta.LastIndex
        
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
                if _, ok := serviceDetails[nodeServiceKey{hc.Node, hc.ServiceID}]; ! ok {
                    // retrieve the service details; don't already have them
                    svcDetails, _, err := self.catalog.Service(hc.ServiceName, "", nil)
                    
                    if err != nil {
                        log.Errorf("error retrieving services: %v", err)
                        keepWatching = false
                        break
                    }
                    
                    for _, svcDetail := range svcDetails {
                        serviceDetails[nodeServiceKey{svcDetail.Node, svcDetail.ServiceID}] = svcDetail
                    }
                }
                
                result.Tags = serviceDetails[nodeServiceKey{hc.Node, hc.ServiceID}].ServiceTags
            }
            
            results = append(results, result)
        }
        
        if keepWatching {
            log.Debug("sending health results")
            select {
                case resultsChan <- results:
                    // successfully sent results
                
                case <-resultsChan:
                    // any value, but probably nil
                    // break out of loop, which will then close the channel
                    keepWatching = false
            }
        }
    }
    
    log.Info("health results watch stopped")
    close(resultsChan)
}
