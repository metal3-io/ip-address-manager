/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
)

// GetIPAddress renders the IP address, taking the index, offset and step into
// account, it is IP version agnostic.
func GetIPAddress(entry Pool, index int) (IPAddressStr, error) {
	if entry.Start == nil && entry.Subnet == nil {
		return "", errors.New("either Start or Subnet is required for ipAddress")
	}
	var ip net.IP
	var err error
	var ipNet *net.IPNet
	offset := index

	// If start is given, use it to add the offset.
	if entry.Start != nil {
		var endIP net.IP
		if entry.End != nil {
			endIP = net.ParseIP(string(*entry.End))
		}
		ip, err = addOffsetToIP(net.ParseIP(string(*entry.Start)), endIP, offset)
		if err != nil {
			return "", err
		}

		// Verify that the IP is in the subnet.
		if entry.Subnet != nil {
			_, ipNet, err = net.ParseCIDR(string(*entry.Subnet))
			if err != nil {
				return "", err
			}
			if !ipNet.Contains(ip) {
				return "", errors.New("IP address out of bonds")
			}
		}

		// If it is not given, use the CIDR ip address and increment the offset by 1.
	} else {
		ip, ipNet, err = net.ParseCIDR(string(*entry.Subnet))
		if err != nil {
			return "", err
		}
		offset++
		ip, err = addOffsetToIP(ip, nil, offset)
		if err != nil {
			return "", err
		}

		// Verify that the ip is in the subnet.
		if !ipNet.Contains(ip) {
			return "", errors.New("IP address out of bonds")
		}
	}
	return IPAddressStr(ip.String()), nil
}

// ValidatePool validates the address-range fields of a single pool entry: that
// any Start/End/Subnet values present are well-formed, and that start <= end
// when both Start and End are set. It is intentionally lenient about presence
// (a Start-only or End-only pool is valid for the sequential strategy), so it
// does not require Start or Subnet to be set and does not bound-check the size.
// It is the shared validation used by both the webhook and GetPoolSize so the
// range/format checks live in a single place.
func ValidatePool(entry Pool) error {
	var startIP, endIP net.IP

	if entry.Start != nil {
		startIP = net.ParseIP(string(*entry.Start))
		if startIP == nil {
			return fmt.Errorf("invalid Start IP %q", *entry.Start)
		}
	}
	if entry.End != nil {
		endIP = net.ParseIP(string(*entry.End))
		if endIP == nil {
			return fmt.Errorf("invalid End IP %q", *entry.End)
		}
	}
	if entry.Subnet != nil {
		if _, _, err := net.ParseCIDR(string(*entry.Subnet)); err != nil {
			return fmt.Errorf("invalid Subnet %q: %w", *entry.Subnet, err)
		}
	}
	if startIP != nil && endIP != nil {
		if new(big.Int).SetBytes(startIP.To16()).Cmp(new(big.Int).SetBytes(endIP.To16())) > 0 {
			return fmt.Errorf("end IP %s is before start IP %s", endIP, startIP)
		}
	}
	return nil
}

// GetPoolSize returns the number of indexable IP addresses in the given pool
// entry, matching the index space accepted by GetIPAddress: GetIPAddress(entry, i)
// is valid for i in [0, size) and returns an error for i >= size. Unlike
// ValidatePool, it requires the pool to be bounded (Start with End/Subnet, or
// Subnet only) so that a finite size can be computed.
func GetPoolSize(entry Pool) (int, error) {
	if err := ValidatePool(entry); err != nil {
		return 0, err
	}
	if entry.Start == nil && entry.Subnet == nil {
		return 0, errors.New("either Start or Subnet is required for ipAddress")
	}

	var startIP, endIP net.IP
	inclusiveStart := true

	switch {
	case entry.Start != nil && entry.End != nil:
		startIP = net.ParseIP(string(*entry.Start))
		endIP = net.ParseIP(string(*entry.End))
	case entry.Start != nil && entry.Subnet != nil:
		_, ipNet, err := net.ParseCIDR(string(*entry.Subnet))
		if err != nil {
			return 0, err
		}
		startIP = net.ParseIP(string(*entry.Start))
		endIP = lastIPInSubnet(ipNet)
	case entry.Start != nil:
		return 0, errors.New("pool with Start requires End or Subnet to determine size")
	default:
		_, ipNet, err := net.ParseCIDR(string(*entry.Subnet))
		if err != nil {
			return 0, err
		}
		// GetIPAddress with Subnet-only maps index 0 to network+1, so the
		// network address itself is excluded from the index space.
		startIP = ipNet.IP
		endIP = lastIPInSubnet(ipNet)
		inclusiveStart = false
	}

	s := new(big.Int).SetBytes(startIP.To16())
	e := new(big.Int).SetBytes(endIP.To16())
	diff := new(big.Int).Sub(e, s)
	if diff.Sign() < 0 {
		return 0, fmt.Errorf("end IP %s is before start IP %s", endIP, startIP)
	}
	if inclusiveStart {
		diff.Add(diff, big.NewInt(1))
	}
	if !diff.IsInt64() || diff.Int64() > math.MaxInt {
		return 0, errors.New("pool size exceeds int range")
	}
	return int(diff.Int64()), nil
}

// lastIPInSubnet returns the highest address contained in the given CIDR.
func lastIPInSubnet(n *net.IPNet) net.IP {
	last := make(net.IP, len(n.IP))
	for i := range n.IP {
		last[i] = n.IP[i] | ^n.Mask[i]
	}
	return last
}

// addOffsetToIP computes the value of the IP address with the offset. It is
// IP version agnostic
// Note that if the resulting IP address is in the format ::ffff:xxxx:xxxx then
// ip.String will fail to select the correct type of ip.
func addOffsetToIP(ip, endIP net.IP, offset int) (net.IP, error) {
	ip4 := false
	if ip.To4() != nil {
		ip4 = true
	}

	// Create big integers.
	IPInt := big.NewInt(0)
	OffsetInt := big.NewInt(int64(offset))

	// Transform the ip into an int. (big endian function).
	IPInt = IPInt.SetBytes(ip)

	// add the two integers.
	IPInt = IPInt.Add(IPInt, OffsetInt)

	// return the bytes list.
	IPBytes := IPInt.Bytes()

	IPBytesLen := len(IPBytes)

	// Verify that the IPv4 or IPv6 fulfills theirs constraints.
	if (ip4 && IPBytesLen > 6 && IPBytes[4] != 255 && IPBytes[5] != 255) ||
		(!ip4 && IPBytesLen > 16) {
		return nil, fmt.Errorf("IP address overflow for : %s", ip.String())
	}

	// transform the end ip into an Int to compare.
	if endIP != nil {
		endIPInt := big.NewInt(0)
		endIPInt = endIPInt.SetBytes(endIP)
		// Computed IP is higher than the end IP.
		if IPInt.Cmp(endIPInt) > 0 {
			return nil, fmt.Errorf("IP address out of bonds for : %s", ip.String())
		}
	}

	// COpy the output back into an ip.
	copy(ip[16-IPBytesLen:], IPBytes)
	return ip, nil
}
