package consulmocks

import (
    "github.com/stretchr/testify/mock"
    "github.com/armon/consul-api"
)

type MockHealth struct {
    mock.Mock
}

func (m *MockHealth) State(state string, q *consulapi.QueryOptions) ([]*consulapi.HealthCheck, *consulapi.QueryMeta, error) {
    ret := m.Called(state, q)

    r0 := ret.Get(0).([]*consulapi.HealthCheck)
    r1 := ret.Get(1).(*consulapi.QueryMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}
