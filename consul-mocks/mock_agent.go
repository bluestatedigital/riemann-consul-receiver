package consulmocks

import (
    "github.com/stretchr/testify/mock"
    "github.com/armon/consul-api"
)

type MockAgent struct {
    mock.Mock
}

func (m *MockAgent) Self() (map[string]map[string]interface{}, error) {
    ret := m.Called()

    r0 := ret.Get(0).(map[string]map[string]interface{})
    r1 := ret.Error(1)

    return r0, r1
}

func (m *MockAgent) ServiceRegister(service *consulapi.AgentServiceRegistration) error {
    ret := m.Called(service)

    r0 := ret.Error(0)

    return r0
}

func (m *MockAgent) PassTTL(checkID, note string) error {
    ret := m.Called(checkID, note)

    r0 := ret.Error(0)

    return r0
}
