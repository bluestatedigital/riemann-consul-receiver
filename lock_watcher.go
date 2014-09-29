package main

import (
    "time"
    "fmt"
    
    log "github.com/Sirupsen/logrus"
    "github.com/armon/consul-api"
)

type LockWatcher struct {
    agent   ConsulAgent
    session ConsulSession
    kv      ConsulKV
    health  ConsulHealth
    
    nodeName string

    serviceName string
    
    keyPath      string
    keyModifyIdx uint64
    
    updateInterval time.Duration
    lockDelay      time.Duration

    sessionID     string
    healthWaitIdx uint64
}

func NewLockWatcher(
    agent   ConsulAgent,
    session ConsulSession,
    kv      ConsulKV,
    health  ConsulHealth,
    
    updateInterval time.Duration,
    lockDelay      time.Duration,
    serviceName    string,
    keyPath        string,
) (*LockWatcher, error) {
    if updateInterval <= lockDelay {
        return nil, fmt.Errorf("update interval must be greater than lock delay")
    }
    
    agentInfo, err := agent.Self()

    if err != nil {
        return nil, err
    }
    
    rcr := &LockWatcher{
        agent:   agent,
        session: session,
        kv:      kv,
        health:  health,

        nodeName: agentInfo["Config"]["NodeName"].(string),

        serviceName: serviceName,
        keyPath:     keyPath,
        
        updateInterval: updateInterval,
        lockDelay:      lockDelay,
    }
    
    return rcr, nil
}

func (self *LockWatcher) RegisterService() error {
    // make TTL three times the update interval
    checkTtl := fmt.Sprintf("%ds", int((self.updateInterval * 3) / time.Second))

    return self.agent.ServiceRegister(&consulapi.AgentServiceRegistration{
        ID:    self.serviceName,
        Name:  self.serviceName,
        Check: &consulapi.AgentServiceCheck{
            TTL: checkTtl,
        },
    })
}

func (self *LockWatcher) InitSession() (string, error) {
    log.Debug("initializing session")
    
    self.sessionID = ""
    
    sess := self.session

    sessions, _, err := sess.List(nil)
    if err != nil {
        return "", fmt.Errorf("unable to retrieve list of sessions: %v", err)
    }
    
    for _, sessionEntry := range sessions {
        if (sessionEntry.Node == self.nodeName) && (sessionEntry.Name == self.serviceName) {
            self.sessionID = sessionEntry.ID
            
            log.WithFields(log.Fields{
                "session": self.sessionID,
            }).Debug("found existing session")
            
            break
        }
    }
    
    if self.sessionID == "" {
        log.Info("creating session")
        
        // Unexpected response code: 500 (Check 'service:riemann-consul-receiver' is in critical state)
        // so we'll tickle the health check first
        self.UpdateHealthCheck()
                
        sessionID, _, err := sess.Create(
            &consulapi.SessionEntry{
                Name: self.serviceName,
                LockDelay: self.lockDelay,
                Checks: []string{
                    "serfHealth",
                    "service:" + self.serviceName,
                },
            },
            nil,
        )
        
        if err != nil {
            return "", fmt.Errorf("unable to create session: %v", err)
        }
        
        self.sessionID = sessionID
    }
    
    log.WithFields(log.Fields{
        "session": self.sessionID,
    }).Info("have session")
    
    return self.sessionID, nil
}

func (self *LockWatcher) DestroySession() {
    log.WithFields(log.Fields{
        "session": self.sessionID,
    }).Info("destroying session")
    
    self.session.Destroy(self.sessionID, nil)
}

func (self *LockWatcher) UpdateHealthCheck() error {
    return self.agent.PassTTL("service:" + self.serviceName, "")
}

func (self *LockWatcher) AcquireLock() (bool, error) {
    // verify session's still valid
    sessionEntry, _, err := self.session.Info(self.sessionID, nil)
    
    if err != nil {
        return false, err
    }

    if sessionEntry == nil {
        return false, fmt.Errorf("session %s is no longer valid", self.sessionID)
    }
    
    kvp, queryMeta, err := self.kv.Get(self.keyPath, &consulapi.QueryOptions{
        WaitIndex: self.keyModifyIdx,
        WaitTime: self.lockDelay,
    })
    
    if err != nil {
        return false, err
    }
    
    isLocked := (kvp != nil) && (kvp.Session != "")
    lockedByUs := isLocked && (kvp.Session == self.sessionID)
    self.keyModifyIdx = queryMeta.LastIndex

    if ! isLocked {
        lockedByUs, _, err = self.kv.Acquire(&consulapi.KVPair{
            Key: self.keyPath,
            Session: self.sessionID,
        }, nil)
    }
    
    return lockedByUs, err
}

func (self *LockWatcher) WatchLock(watchChan chan<- interface{}) {
    lockedByUs := true
    
    for lockedByUs {
        kvp, queryMeta, err := self.kv.Get(self.keyPath, &consulapi.QueryOptions{
            WaitIndex: self.keyModifyIdx,
            WaitTime: time.Minute,
        })
        
        if err == nil {
            isLocked := (kvp != nil) && (kvp.Session != "")
            lockedByUs = isLocked && (kvp.Session == self.sessionID)
            self.keyModifyIdx = queryMeta.LastIndex
        } else {
            log.Errorf("unable to check key: %v", err)
        }
    }
    
    close(watchChan)
}

func (self *LockWatcher) ReleaseLock() error {
    _, _, err := self.kv.Release(
        &consulapi.KVPair{
            Key: self.keyPath,
            Session: self.sessionID,
        },
        nil,
    )
    
    return err
}
