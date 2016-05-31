package consulmocks

import (
    "github.com/stretchr/testify/mock"
    "github.com/armon/consul-api"
)

type MockKV struct {
    mock.Mock
}

func (m *MockKV) Acquire(p *consulapi.KVPair, q *consulapi.WriteOptions) (bool, *consulapi.WriteMeta, error) {
    ret := m.Called(p, q)

    r0 := ret.Get(0).(bool)
    
    var retWm *consulapi.WriteMeta = nil
    
    if ret.Get(1) != nil {
        retWm = ret.Get(1).(*consulapi.WriteMeta)
    }

    r2 := ret.Error(2)

    return r0, retWm, r2
}

func (m *MockKV) Get(key string, q *consulapi.QueryOptions) (*consulapi.KVPair, *consulapi.QueryMeta, error) {
    ret := m.Called(key, q)

    var retKvp *consulapi.KVPair = nil
    var retQm *consulapi.QueryMeta = nil
        
    if ret.Get(0) != nil {
        retKvp = ret.Get(0).(*consulapi.KVPair)
    }
    
    if ret.Get(1) != nil {
        retQm = ret.Get(1).(*consulapi.QueryMeta)
    }

    r2 := ret.Error(2)

    return retKvp, retQm, r2
}

func (m *MockKV) Release(p *consulapi.KVPair, q *consulapi.WriteOptions) (bool, *consulapi.WriteMeta, error) {
    ret := m.Called(p, q)

    r0 := ret.Get(0).(bool)
    r1 := ret.Get(1).(*consulapi.WriteMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}
