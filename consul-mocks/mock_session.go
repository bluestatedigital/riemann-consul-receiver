package consulmocks

import (
    "github.com/stretchr/testify/mock"
    "github.com/armon/consul-api"
)

type MockSession struct {
    mock.Mock
}

func (m *MockSession) List(q *consulapi.QueryOptions) ([]*consulapi.SessionEntry, *consulapi.QueryMeta, error) {
    ret := m.Called(q)

    r0 := ret.Get(0).([]*consulapi.SessionEntry)
    r1 := ret.Get(1).(*consulapi.QueryMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}

func (m *MockSession) Create(se *consulapi.SessionEntry, q *consulapi.WriteOptions) (string, *consulapi.WriteMeta, error) {
    ret := m.Called(se, q)

    r0 := ret.Get(0).(string)
    r1 := ret.Get(1).(*consulapi.WriteMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}

func (m *MockSession) Info(id string, q *consulapi.QueryOptions) (*consulapi.SessionEntry, *consulapi.QueryMeta, error) {
    ret := m.Called(id, q)
    
    var retSess *consulapi.SessionEntry = nil
    var retQm *consulapi.QueryMeta = nil
        
    if ret.Get(0) != nil {
        retSess = ret.Get(0).(*consulapi.SessionEntry)
    }
    
    if ret.Get(1) != nil {
        retQm = ret.Get(1).(*consulapi.QueryMeta)
    }
    
    r2 := ret.Error(2)

    return retSess, retQm, r2
}

func (m *MockSession) Destroy(id string, q *consulapi.WriteOptions) (*consulapi.WriteMeta, error) {
    ret := m.Called(id, q)

    r0 := ret.Get(0).(*consulapi.WriteMeta)
    r1 := ret.Error(1)

    return r0, r1
}
