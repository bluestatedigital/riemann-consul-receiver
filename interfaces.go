package main

import (
    "github.com/amir/raidman"
    "github.com/armon/consul-api"
)

type RiemannClient interface {
    Send(*raidman.Event) error
}

type ConsulAgent interface {
    Self() (map[string]map[string]interface{}, error)
    ServiceRegister(service *consulapi.AgentServiceRegistration) error
    PassTTL(checkID, note string) error
}

type ConsulSession interface {
    List(q *consulapi.QueryOptions) ([]*consulapi.SessionEntry, *consulapi.QueryMeta, error)
    Create(se *consulapi.SessionEntry, q *consulapi.WriteOptions) (string, *consulapi.WriteMeta, error)
    Info(id string, q *consulapi.QueryOptions) (*consulapi.SessionEntry, *consulapi.QueryMeta, error)
    Destroy(id string, q *consulapi.WriteOptions) (*consulapi.WriteMeta, error)
}

type ConsulKV interface {
    Acquire(p *consulapi.KVPair, q *consulapi.WriteOptions) (bool, *consulapi.WriteMeta, error)
    Get(key string, q *consulapi.QueryOptions) (*consulapi.KVPair, *consulapi.QueryMeta, error)
    Release(p *consulapi.KVPair, q *consulapi.WriteOptions) (bool, *consulapi.WriteMeta, error)
}

type ConsulHealth interface {
    State(state string, q *consulapi.QueryOptions) ([]*consulapi.HealthCheck, *consulapi.QueryMeta, error)
}
