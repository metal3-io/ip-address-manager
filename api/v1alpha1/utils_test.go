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

			result, err := addOffsetToIP(testIP, testEndIP, tc.offset)
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
	)

})
