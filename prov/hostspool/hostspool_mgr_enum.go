// Code generated by go-enum
// DO NOT EDIT!

package hostspool

import (
	"fmt"
	"strings"
)

const (
	// HostStatusFree is a HostStatus of type Free
	HostStatusFree HostStatus = iota
	// HostStatusAllocated is a HostStatus of type Allocated
	HostStatusAllocated
)

const _HostStatusName = "FreeAllocated"

var _HostStatusMap = map[HostStatus]string{
	0: _HostStatusName[0:4],
	1: _HostStatusName[4:13],
}

func (i HostStatus) String() string {
	if str, ok := _HostStatusMap[i]; ok {
		return str
	}
	return fmt.Sprintf("HostStatus(%d)", i)
}

var _HostStatusValue = map[string]HostStatus{
	_HostStatusName[0:4]:                   0,
	strings.ToLower(_HostStatusName[0:4]):  0,
	_HostStatusName[4:13]:                  1,
	strings.ToLower(_HostStatusName[4:13]): 1,
}

// ParseHostStatus attempts to convert a string to a HostStatus
func ParseHostStatus(name string) (HostStatus, error) {
	if x, ok := _HostStatusValue[name]; ok {
		return HostStatus(x), nil
	}
	return HostStatus(0), fmt.Errorf("%s is not a valid HostStatus", name)
}
