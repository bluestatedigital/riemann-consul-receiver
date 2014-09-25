package main

import (
    "time"
    "fmt"
    
    log "github.com/Sirupsen/logrus"
    "github.com/armon/consul-api"
)

type ConsulReceiver struct {
    consul   ConsulClient
    nodeName string

    serviceName string
    
    keyPath      string
    keyModifyIdx uint64
    
    updateInterval time.Duration

    session       *consulapi.SessionEntry
    healthWaitIdx uint64
}

func NewConsulReceiver(
    consul ConsulClient,
    updateInterval time.Duration,
    serviceName string,
    keyPath string,
) (*ConsulReceiver, error) {
    agentInfo, err := consul.Agent().Self()

    if err != nil {
        return nil, err
    }
    
    rcr := &ConsulReceiver{
        consul: consul,
        nodeName: agentInfo["Config"]["NodeName"].(string),

        serviceName: serviceName,
        keyPath: keyPath,
        updateInterval: updateInterval,
    }
    
    return rcr, nil
}

func (self *ConsulReceiver) RegisterService() error {
    // make TTL three times the update interval
    checkTtl := fmt.Sprintf("%ds", int((self.updateInterval * 3) / time.Second))

    return self.consul.Agent().ServiceRegister(&consulapi.AgentServiceRegistration{
        ID: self.serviceName,
        Name: self.serviceName,
        Check: &consulapi.AgentServiceCheck{
            TTL: checkTtl,
        },
    })
}

func (self *ConsulReceiver) InitSession() (*consulapi.SessionEntry, error) {
    log.Debug("initializing session")
    
    self.session = nil
    
    sess := self.consul.Session()
    
    sessions, _, err := sess.List(nil)
    if err != nil {
        return nil, fmt.Errorf("unable to retrieve list of sessions: %v", err)
    }
    
    for _, sessionEntry := range sessions {
        if (sessionEntry.Node == self.nodeName) && (sessionEntry.Name == self.serviceName) {
            self.session = sessionEntry
            
            log.WithFields(log.Fields{
                "session": self.session.ID,
            }).Debug("found existing session")
            
            break
        }
    }
    
    if self.session == nil {
        log.Info("creating session")
        
        // Unexpected response code: 500 (Check 'service:riemann-consul-receiver' is in critical state)
        // so we'll tickle the health check first
        self.UpdateHealthCheck()
                
        sessionId, _, err := sess.Create(
            &consulapi.SessionEntry{
                Name: self.serviceName,
                Checks: []string{
                    "serfHealth",
                    "service:" + self.serviceName,
                },
            },
            nil,
        )
        
        if err != nil {
            return nil, fmt.Errorf("unable to create session: %v", err)
        }

        sessionEntry, _, err := sess.Info(sessionId, nil)
        if err != nil {
            return nil, fmt.Errorf("unable to retrieve newly-created session: %v", err)
        }

        self.session = sessionEntry
    }
    
    log.WithFields(log.Fields{
        "session": self.session.ID,
    }).Debug("have session")
    
    return self.session, nil
}

func (self *ConsulReceiver) DestroySession() {
    log.WithFields(log.Fields{
        "session": self.session.ID,
    }).Info("destroying session")
    
    self.consul.Session().Destroy(self.session.ID, nil)
}

func (self *ConsulReceiver) UpdateHealthCheck() error {
    return self.consul.Agent().PassTTL("service:" + self.serviceName, "")
}

func (self *ConsulReceiver) AcquireLock(waitDuration time.Duration) (bool, error) {
    /*
        if have lock
            return true
        
        if someone else has lock
            wait for change
            
            if still acquired
                return false
        
        if acquire lock
            return true
        else
            return false
    */

    log.WithFields(log.Fields{
        "key": self.keyPath,
        "session": self.session.ID,
    }).Debug("checking for lock")
    
    kvp, queryMeta, err := self.consul.KV().Get(self.keyPath, nil)
    
    if err != nil {
        return false, fmt.Errorf("unable to retrieve key: %v", err)
    }
    
    keyModifyIdx := queryMeta.LastIndex

    isLocked := (kvp != nil) && (kvp.Session != "")
    haveLock := isLocked && (kvp.Session == self.session.ID)
    
    if haveLock {
        log.WithFields(log.Fields{
            "key": self.keyPath,
            "session": self.session.ID,
        }).Debug("the lock is mine.  mine! <manical laugh>")
        
        return true, nil
    }
    
    if isLocked {
        log.WithFields(log.Fields{
            "key": self.keyPath,
            "session": self.session.ID,
        }).Debugf("key locked by session %s", kvp.Session)

        // someone else has lock
        kvp, _, err := self.consul.KV().Get(self.keyPath, &consulapi.QueryOptions{
            WaitIndex: keyModifyIdx,
            WaitTime: waitDuration,
        })
        
        if err != nil {
            return false, fmt.Errorf("unable to retrieve key: %v", err)
        }

        if (kvp != nil) && (kvp.Session != "") {
            // someone else *still* has the lock
            log.WithFields(log.Fields{
                "key": self.keyPath,
                "session": self.session.ID,
            }).Debugf("key still locked by session %s", kvp.Session)

            return false, nil
        }
    }
    
    // not locked by anybody; grab it for ourselves
    log.WithFields(log.Fields{
        "key": self.keyPath,
        "session": self.session.ID,
    }).Debug("acquiring lock")

    kvp = &consulapi.KVPair{
        Key: self.keyPath,
        Session: self.session.ID,
    }

    haveLock, _, err = self.consul.KV().Acquire(kvp, nil)
    
    if err != nil {
        return false, err
    }

    return haveLock, nil
}

func (self *ConsulReceiver) ReleaseLock() error {
    _, _, err := self.consul.KV().Release(
        &consulapi.KVPair{
            Key: self.keyPath,
            Session: self.session.ID,
        },
        nil,
    )
    
    return err
}

func (self *ConsulReceiver) GetHealthResults(waitTime time.Duration) ([]*consulapi.HealthCheck, error) {
    healthChecks, queryMeta, err := self.consul.Health().State("any", &consulapi.QueryOptions{
        WaitIndex: self.healthWaitIdx,
        WaitTime:  waitTime,
    })
    
    if err != nil {
        return nil, fmt.Errorf("error retrieving health results: %v", err)
    }

    self.healthWaitIdx = queryMeta.LastIndex
    
    return healthChecks, nil
}
