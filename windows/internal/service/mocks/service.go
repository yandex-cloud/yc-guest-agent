// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	svc "golang.org/x/sys/windows/svc"
)

// Service is an autogenerated mock type for the service type
type Service struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *Service) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Control provides a mock function with given fields: c
func (_m *Service) Control(c svc.Cmd) (svc.Status, error) {
	ret := _m.Called(c)

	var r0 svc.Status
	if rf, ok := ret.Get(0).(func(svc.Cmd) svc.Status); ok {
		r0 = rf(c)
	} else {
		r0 = ret.Get(0).(svc.Status)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(svc.Cmd) error); ok {
		r1 = rf(c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Delete provides a mock function with given fields:
func (_m *Service) Delete() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Query provides a mock function with given fields:
func (_m *Service) Query() (svc.Status, error) {
	ret := _m.Called()

	var r0 svc.Status
	if rf, ok := ret.Get(0).(func() svc.Status); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(svc.Status)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Start provides a mock function with given fields: args
func (_m *Service) Start(args ...string) error {
	_va := make([]interface{}, len(args))
	for _i := range args {
		_va[_i] = args[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(...string) error); ok {
		r0 = rf(args...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
