// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package flags

import (
	"fmt"
	"net"
	"strconv"
)

func ParseIPV4Addr(addrStr string) (ip [4]byte, port uint16, err error) {
	hostStr, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		panic(err)
	}

	ipV := net.ParseIP(hostStr)
	if ipV == nil || ipV.To4() == nil {
		return ip, port, fmt.Errorf("invalid ip address %q", hostStr)
	}

	port64, err := strconv.ParseUint(portStr, 0, 16)
	if err != nil {
		return ip, port, err
	}

	copy(ip[:], ipV.To4()[:])
	port = uint16(port64)

	return ip, port, nil
}
