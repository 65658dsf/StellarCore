// Copyright 2023 The frp Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestServerConfigComplete(t *testing.T) {
	require := require.New(t)
	c := &ServerConfig{}
	c.Complete()

	require.EqualValues("token", c.Auth.Method)
	require.Equal(true, lo.FromPtr(c.Transport.TCPMux))
	require.Equal(true, lo.FromPtr(c.DetailedErrorsToClient))
	require.Equal(TrafficDecisionModePrecision, c.TrafficMonitor.DecisionMode)
	require.Equal(80, c.TrafficMonitor.InspectTimeoutMS)
	require.Equal(512, c.TrafficMonitor.InspectMaxBytes)
}

func TestTrafficMonitorCompleteNormalizesProtocols(t *testing.T) {
	require := require.New(t)

	c := &ServerConfig{
		TrafficMonitor: TrafficMonitorConfig{
			DecisionMode: TrafficDecisionModeBalanced,
			VPNProtocols: []string{"hy2", "openvpn-tcp", "ipsec", "openvpn"},
		},
	}
	c.Complete()

	require.Equal(120, c.TrafficMonitor.InspectTimeoutMS)
	require.Equal(1024, c.TrafficMonitor.InspectMaxBytes)
	require.Equal([]string{"hysteria2", "openvpn", "ikev2"}, c.TrafficMonitor.VPNProtocols)
}
