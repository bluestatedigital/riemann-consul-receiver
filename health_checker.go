package main

import (
    "time"
    log "github.com/Sirupsen/logrus"

    "github.com/armon/consul-api"
)

type HealthChecker struct {
    health         ConsulHealth
    updateInterval time.Duration
}

func NewHealthChecker(health ConsulHealth, updateInterval time.Duration) *HealthChecker {
    return &HealthChecker{
        health: health,
        updateInterval: updateInterval,
    }
}

func (self *HealthChecker) WatchHealthResults(resultsChan chan []consulapi.HealthCheck) {
    waitIdx := uint64(0)
    keepWatching := true
    
    for keepWatching {
        log.Debugf("retrieving health results; WaitIndex=%d", waitIdx)

        healthChecks, queryMeta, err := self.health.State("any", &consulapi.QueryOptions{
            WaitIndex: waitIdx,
            WaitTime:  self.updateInterval,
        })
        
        if err != nil {
            log.Errorf("error retrieving health results: %v", err)
            break
        }
        
        waitIdx = queryMeta.LastIndex
        
        var results []consulapi.HealthCheck
        for _, hc := range healthChecks {
            results = append(results, *hc)
        }
        
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
    
    log.Info("health results watch stopped")
    close(resultsChan)
}
