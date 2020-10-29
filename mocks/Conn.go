// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	rados "github.com/ceph/go-ceph/rados"
	mock "github.com/stretchr/testify/mock"
)

// Conn is an autogenerated mock type for the Conn type
type Conn struct {
	mock.Mock
}

// GetPoolStats provides a mock function with given fields: _a0
func (_m *Conn) GetPoolStats(_a0 string) (*rados.PoolStat, error) {
	ret := _m.Called(_a0)

	var r0 *rados.PoolStat
	if rf, ok := ret.Get(0).(func(string) *rados.PoolStat); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*rados.PoolStat)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MgrCommand provides a mock function with given fields: _a0
func (_m *Conn) MgrCommand(_a0 [][]byte) ([]byte, string, error) {
	ret := _m.Called(_a0)

	var r0 []byte
	if rf, ok := ret.Get(0).(func([][]byte) []byte); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 string
	if rf, ok := ret.Get(1).(func([][]byte) string); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Get(1).(string)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func([][]byte) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MonCommand provides a mock function with given fields: _a0
func (_m *Conn) MonCommand(_a0 []byte) ([]byte, string, error) {
	ret := _m.Called(_a0)

	var r0 []byte
	if rf, ok := ret.Get(0).(func([]byte) []byte); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 string
	if rf, ok := ret.Get(1).(func([]byte) string); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Get(1).(string)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func([]byte) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}
