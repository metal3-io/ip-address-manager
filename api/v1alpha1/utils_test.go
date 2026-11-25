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
	"math"
	"math/big"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("IPPool manager", func() {
	type testCaseGetIPAddress struct {
		ipAddress   Pool
		index       int
		expectError bool
		expectedIP  IPAddressStr
	}

	DescribeTable("Test getIPAddress",
		func(tc testCaseGetIPAddress) {
			result, err := GetIPAddress(tc.ipAddress, tc.index)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.expectedIP))
			}
		},
		Entry("Empty Start and Subnet", testCaseGetIPAddress{
			ipAddress:   Pool{},
			index:       1,
			expectError: true,
		}),
		Entry("Start set, no end or subnet", testCaseGetIPAddress{
			ipAddress: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
			},
			index:      1,
			expectedIP: IPAddressStr("192.168.0.11"),
		}),
		Entry("Start set, end set, subnet unset", testCaseGetIPAddress{
			ipAddress: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
				End:   (*IPAddressStr)(ptr.To("192.168.0.100")),
			},
			index:      1,
			expectedIP: IPAddressStr("192.168.0.11"),
		}),
		Entry("Start set, end set, subnet unset, out of bound", testCaseGetIPAddress{
			ipAddress: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
				End:   (*IPAddressStr)(ptr.To("192.168.0.100")),
			},
			index:       100,
			expectError: true,
		}),
		Entry("Start set, end unset, subnet set", testCaseGetIPAddress{
			ipAddress: Pool{
				Start:  (*IPAddressStr)(ptr.To("192.168.0.10")),
				Subnet: (*IPSubnetStr)(ptr.To("192.168.0.0/24")),
			},
			index:      1,
			expectedIP: IPAddressStr("192.168.0.11"),
		}),
		Entry("Start set, end unset, subnet set, out of bound", testCaseGetIPAddress{
			ipAddress: Pool{
				Start:  (*IPAddressStr)(ptr.To("192.168.0.10")),
				Subnet: (*IPSubnetStr)(ptr.To("192.168.0.0/24")),
			},
			index:       250,
			expectError: true,
		}),
		Entry("Start set, end unset, subnet empty", testCaseGetIPAddress{
			ipAddress: Pool{
				Start:  (*IPAddressStr)(ptr.To("192.168.0.10")),
				Subnet: (*IPSubnetStr)(ptr.To("")),
			},
			index:       1,
			expectError: true,
		}),
		Entry("subnet empty", testCaseGetIPAddress{
			ipAddress: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("")),
			},
			index:       1,
			expectError: true,
		}),
		Entry("Start unset, end unset, subnet set", testCaseGetIPAddress{
			ipAddress: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("192.168.0.10/24")),
			},
			index:      1,
			expectedIP: IPAddressStr("192.168.0.12"),
		}),
		Entry("Start unset, end unset, subnet set, out of bound", testCaseGetIPAddress{
			ipAddress: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("192.168.0.10/24")),
			},
			index:       250,
			expectError: true,
		}),
	)

	type testCaseGetIPAddressBigIndex struct {
		ipAddress     Pool
		index         *big.Int
		expectedError string
		expectedIP    IPAddressStr
	}

	// pow2 returns 2^n + delta.
	pow2 := func(n uint, delta int64) *big.Int {
		return new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), n), big.NewInt(delta))
	}

	DescribeTable("Test GetIPAddressBigIndex",
		func(tc testCaseGetIPAddressBigIndex) {
			result, err := GetIPAddressBigIndex(tc.ipAddress, tc.index)
			if tc.expectedError != "" {
				Expect(err).To(MatchError(tc.expectedError))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.expectedIP))
			}
			// GetIPAddress gives the same result for an index that fits in an int.
			if tc.index.IsInt64() && tc.index.Int64() <= math.MaxInt {
				intResult, intErr := GetIPAddress(tc.ipAddress, int(tc.index.Int64()))
				Expect(intResult).To(Equal(result))
				Expect(intErr != nil).To(Equal(err != nil))
			}
		},
		Entry("Start unset, end unset, subnet set, small index", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("2001:db8::/64")),
			},
			index:      big.NewInt(1),
			expectedIP: IPAddressStr("2001:db8::2"),
		}),
		Entry("Start unset, end unset, subnet set, index 1<<63 - 1", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("2001:db8::/64")),
			},
			index:      pow2(63, -1),
			expectedIP: IPAddressStr("2001:db8:0:0:8000::"),
		}),
		Entry("Start unset, end unset, subnet set, index 1<<63", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("2001:db8::/64")),
			},
			index:      pow2(63, 0),
			expectedIP: IPAddressStr("2001:db8::8000:0:0:1"),
		}),
		Entry("Start unset, end unset, subnet set, last address", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("2001:db8::/64")),
			},
			index:      pow2(64, -2),
			expectedIP: IPAddressStr("2001:db8::ffff:ffff:ffff:ffff"),
		}),
		Entry("Start unset, end unset, subnet set, out of bound", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("2001:db8::/64")),
			},
			index:         pow2(64, -1),
			expectedError: "IP address out of bounds",
		}),
		Entry("Start set, end set, subnet unset, index 1<<63", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Start: (*IPAddressStr)(ptr.To("2001:db8::")),
				End:   (*IPAddressStr)(ptr.To("2001:db8::8000:0:0:0")),
			},
			index:      pow2(63, 0),
			expectedIP: IPAddressStr("2001:db8:0:0:8000::"),
		}),
		Entry("Start set, end set, subnet unset, out of bound", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Start: (*IPAddressStr)(ptr.To("2001:db8::")),
				End:   (*IPAddressStr)(ptr.To("2001:db8::8000:0:0:0")),
			},
			index:         pow2(63, 1),
			expectedError: "IP address out of bounds for : 2001:db8::",
		}),
		Entry("Start set, end unset, subnet set, last address", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Start:  (*IPAddressStr)(ptr.To("2001:db8::10")),
				Subnet: (*IPSubnetStr)(ptr.To("2001:db8::/64")),
			},
			index:      pow2(64, -17),
			expectedIP: IPAddressStr("2001:db8::ffff:ffff:ffff:ffff"),
		}),
		Entry("Start set, end unset, subnet set, out of bound", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Start:  (*IPAddressStr)(ptr.To("2001:db8::10")),
				Subnet: (*IPSubnetStr)(ptr.To("2001:db8::/64")),
			},
			index:         pow2(64, -16),
			expectedError: "IP address out of bounds",
		}),
		Entry("Start set, no end or subnet, IPv6 maximum address", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Start: (*IPAddressStr)(ptr.To("ffff:ffff:ffff:ffff::")),
			},
			index:      pow2(64, -1),
			expectedIP: IPAddressStr("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"),
		}),
		Entry("Start set, no end or subnet, IPv6 overflow", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Start: (*IPAddressStr)(ptr.To("ffff:ffff:ffff:ffff::")),
			},
			index:         pow2(64, 0),
			expectedError: "IP address overflow for : ffff:ffff:ffff:ffff::",
		}),
		Entry("Start set, no end or subnet, IPv4 overflow", testCaseGetIPAddressBigIndex{
			ipAddress: Pool{
				Start: (*IPAddressStr)(ptr.To("10.0.0.1")),
			},
			index:         pow2(64, 0),
			expectedError: "IP address overflow for : 10.0.0.1",
		}),
	)

	type testCaseAddOffsetToIP struct {
		ip          string
		endIP       string
		offset      int
		expectedIP  string
		expectError bool
	}

	DescribeTable("Test AddOffsetToIP",
		func(tc testCaseAddOffsetToIP) {
			testIP := net.ParseIP(tc.ip)
			testEndIP := net.ParseIP(tc.endIP)
			expectedIP := net.ParseIP(tc.expectedIP)

			result, err := addOffsetToIP(testIP, testEndIP, big.NewInt(int64(tc.offset)))
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedIP))
			}
		},
		Entry("valid IPv4", testCaseAddOffsetToIP{
			ip:         "192.168.0.10",
			endIP:      "192.168.0.200",
			offset:     10,
			expectedIP: "192.168.0.20",
		}),
		Entry("valid IPv4, no end ip", testCaseAddOffsetToIP{
			ip:         "192.168.0.10",
			offset:     1000,
			expectedIP: "192.168.3.242",
		}),
		Entry("Over bound ipv4", testCaseAddOffsetToIP{
			ip:          "192.168.0.10",
			endIP:       "192.168.0.200",
			offset:      1000,
			expectError: true,
		}),
		Entry("error ipv4", testCaseAddOffsetToIP{
			ip:          "255.255.255.250",
			offset:      10,
			expectError: true,
		}),
		Entry("valid IPv6", testCaseAddOffsetToIP{
			ip:         "2001::10",
			endIP:      "2001::fff0",
			offset:     10,
			expectedIP: "2001::1A",
		}),
		Entry("valid IPv6, no end ip", testCaseAddOffsetToIP{
			ip:         "2001::10",
			offset:     10000,
			expectedIP: "2001::2720",
		}),
		Entry("Over bound ipv6", testCaseAddOffsetToIP{
			ip:          "2001::10",
			endIP:       "2001::00f0",
			offset:      10000,
			expectError: true,
		}),
		Entry("error ipv6", testCaseAddOffsetToIP{
			ip:          "FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFF0",
			offset:      100,
			expectError: true,
		}),

		Entry("IPv4 minimum address", testCaseAddOffsetToIP{
			ip:         "0.0.0.0",
			offset:     1,
			expectedIP: "0.0.0.1",
		}),
		Entry("IPv4 maximum address no overflow", testCaseAddOffsetToIP{
			ip:         "255.255.255.254",
			offset:     1,
			expectedIP: "255.255.255.255",
		}),
		Entry("IPv4 overflow from max address", testCaseAddOffsetToIP{
			ip:          "255.255.255.255",
			offset:      1,
			expectError: true,
		}),
		Entry("IPv4 overflow from high address with large offset", testCaseAddOffsetToIP{
			ip:          "255.255.255.0",
			offset:      256,
			expectError: true,
		}),
		Entry("IPv4 zero offset returns same address", testCaseAddOffsetToIP{
			ip:         "10.0.0.1",
			offset:     0,
			expectedIP: "10.0.0.1",
		}),

		// IPv6 boundary tests
		Entry("IPv6 minimum address", testCaseAddOffsetToIP{
			ip:         "::",
			offset:     1,
			expectedIP: "::1",
		}),
		Entry("IPv6 maximum address no overflow", testCaseAddOffsetToIP{
			ip:         "FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFE",
			offset:     1,
			expectedIP: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		}),
		Entry("IPv6 overflow from max address", testCaseAddOffsetToIP{
			ip:          "FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF",
			offset:      1,
			expectError: true,
		}),
		Entry("IPv6 normal address with offset", testCaseAddOffsetToIP{
			ip:         "2001:db8::1",
			offset:     255,
			expectedIP: "2001:db8::100",
		}),
		Entry("IPv6 zero offset returns same address", testCaseAddOffsetToIP{
			ip:         "fe80::1",
			offset:     0,
			expectedIP: "fe80::1",
		}),
		Entry("IPv6 large address not mistaken for IPv4 overflow", testCaseAddOffsetToIP{
			ip:         "2001:db8::ffff:ffff",
			offset:     1,
			expectedIP: "2001:db8::1:0:0",
		}),
		Entry("IPv6 with end IP within bounds", testCaseAddOffsetToIP{
			ip:         "2001:db8::1",
			endIP:      "2001:db8::ff",
			offset:     10,
			expectedIP: "2001:db8::b",
		}),
		Entry("IPv6 with end IP out of bounds", testCaseAddOffsetToIP{
			ip:          "2001:db8::1",
			endIP:       "2001:db8::ff",
			offset:      1000,
			expectError: true,
		}),
	)

	type testCaseGetPoolSize struct {
		pool         Pool
		expectError  bool
		expectedSize int
	}

	DescribeTable("Test GetPoolSize",
		func(tc testCaseGetPoolSize) {
			size, err := GetPoolSize(tc.pool)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(size).To(Equal(tc.expectedSize))
			}
		},
		Entry("Empty Start and Subnet", testCaseGetPoolSize{
			pool:        Pool{},
			expectError: true,
		}),
		Entry("Start set, end and subnet unset", testCaseGetPoolSize{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
			},
			expectError: true,
		}),
		Entry("Start and End set, inclusive range", testCaseGetPoolSize{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
				End:   (*IPAddressStr)(ptr.To("192.168.0.20")),
			},
			expectedSize: 11,
		}),
		Entry("Start and End set, single address", testCaseGetPoolSize{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
				End:   (*IPAddressStr)(ptr.To("192.168.0.10")),
			},
			expectedSize: 1,
		}),
		Entry("Start and End set, end before start", testCaseGetPoolSize{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.20")),
				End:   (*IPAddressStr)(ptr.To("192.168.0.10")),
			},
			expectError: true,
		}),
		Entry("Start and Subnet set, subnet bounds the end", testCaseGetPoolSize{
			pool: Pool{
				Start:  (*IPAddressStr)(ptr.To("192.168.0.10")),
				Subnet: (*IPSubnetStr)(ptr.To("192.168.0.0/24")),
			},
			expectedSize: 246,
		}),
		Entry("Subnet only excludes network address", testCaseGetPoolSize{
			pool: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("192.168.0.0/24")),
			},
			expectedSize: 255,
		}),
		Entry("Subnet only with host bits set is normalized", testCaseGetPoolSize{
			pool: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("192.168.0.10/24")),
			},
			expectedSize: 255,
		}),
		Entry("Invalid Start IP", testCaseGetPoolSize{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("not-an-ip")),
				End:   (*IPAddressStr)(ptr.To("192.168.0.20")),
			},
			expectError: true,
		}),
		Entry("Invalid End IP", testCaseGetPoolSize{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
				End:   (*IPAddressStr)(ptr.To("not-an-ip")),
			},
			expectError: true,
		}),
		Entry("Invalid Subnet with Start", testCaseGetPoolSize{
			pool: Pool{
				Start:  (*IPAddressStr)(ptr.To("192.168.0.10")),
				Subnet: (*IPSubnetStr)(ptr.To("not-a-cidr")),
			},
			expectError: true,
		}),
		Entry("Invalid Subnet only", testCaseGetPoolSize{
			pool: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("not-a-cidr")),
			},
			expectError: true,
		}),
	)

	type testCaseValidatePool struct {
		pool        Pool
		expectError bool
	}

	DescribeTable("Test ValidatePool",
		func(tc testCaseValidatePool) {
			err := ValidatePool(tc.pool)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		// ValidatePool is lenient about presence: empty, Start-only and End-only
		// pools are valid (the sequential strategy can use an unbounded pool).
		Entry("Empty pool is valid", testCaseValidatePool{
			pool: Pool{},
		}),
		Entry("Start only is valid", testCaseValidatePool{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
			},
		}),
		Entry("End only is valid", testCaseValidatePool{
			pool: Pool{
				End: (*IPAddressStr)(ptr.To("192.168.0.20")),
			},
		}),
		Entry("Start and End set", testCaseValidatePool{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
				End:   (*IPAddressStr)(ptr.To("192.168.0.20")),
			},
		}),
		Entry("Start and Subnet set", testCaseValidatePool{
			pool: Pool{
				Start:  (*IPAddressStr)(ptr.To("192.168.0.10")),
				Subnet: (*IPSubnetStr)(ptr.To("192.168.0.0/24")),
			},
		}),
		Entry("Subnet only", testCaseValidatePool{
			pool: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("192.168.0.0/24")),
			},
		}),
		Entry("End before Start", testCaseValidatePool{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.20")),
				End:   (*IPAddressStr)(ptr.To("192.168.0.10")),
			},
			expectError: true,
		}),
		Entry("Invalid Start IP", testCaseValidatePool{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("not-an-ip")),
				End:   (*IPAddressStr)(ptr.To("192.168.0.20")),
			},
			expectError: true,
		}),
		Entry("Invalid End IP", testCaseValidatePool{
			pool: Pool{
				Start: (*IPAddressStr)(ptr.To("192.168.0.10")),
				End:   (*IPAddressStr)(ptr.To("not-an-ip")),
			},
			expectError: true,
		}),
		Entry("Invalid Subnet", testCaseValidatePool{
			pool: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("not-a-cidr")),
			},
			expectError: true,
		}),
		// A large IPv6 subnet is a valid pool even though its size exceeds the
		// int range; GetPoolSize rejects it but ValidatePool must not, since the
		// sequential strategy can still use it.
		Entry("Large IPv6 subnet is valid", testCaseValidatePool{
			pool: Pool{
				Subnet: (*IPSubnetStr)(ptr.To("2001:db8::/32")),
			},
		}),
	)

})
