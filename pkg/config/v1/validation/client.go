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

package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/samber/lo"

	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
)

func ValidateClientCommonConfig(c *v1.ClientCommonConfig) (Warning, error) {
	var (
		warnings Warning
		errs     error
	)
	if !slices.Contains(SupportedAuthMethods, c.Auth.Method) {
		errs = AppendError(errs, fmt.Errorf("无效的认证方法，可选值为 %v", SupportedAuthMethods))
	}
	if !lo.Every(SupportedAuthAdditionalScopes, c.Auth.AdditionalScopes) {
		errs = AppendError(errs, fmt.Errorf("无效的认证额外作用域，可选值为 %v", SupportedAuthAdditionalScopes))
	}

	if err := validateLogConfig(&c.Log); err != nil {
		errs = AppendError(errs, err)
	}

	if err := validateWebServerConfig(&c.WebServer); err != nil {
		errs = AppendError(errs, err)
	}

	if c.Transport.HeartbeatTimeout > 0 && c.Transport.HeartbeatInterval > 0 {
		if c.Transport.HeartbeatTimeout < c.Transport.HeartbeatInterval {
			errs = AppendError(errs, fmt.Errorf("transport.heartbeatTimeout 不能小于 transport.heartbeatInterval"))
		}
	}

	if !lo.FromPtr(c.Transport.TLS.Enable) {
		checkTLSConfig := func(name string, value string) Warning {
			if value != "" {
				return fmt.Errorf("%s 无效，当 transport.tls.enable 为 false 时不能配置", name)
			}
			return nil
		}

		warnings = AppendError(warnings, checkTLSConfig("transport.tls.certFile", c.Transport.TLS.CertFile))
		warnings = AppendError(warnings, checkTLSConfig("transport.tls.keyFile", c.Transport.TLS.KeyFile))
		warnings = AppendError(warnings, checkTLSConfig("transport.tls.trustedCaFile", c.Transport.TLS.TrustedCaFile))
	}

	if !slices.Contains(SupportedTransportProtocols, c.Transport.Protocol) {
		errs = AppendError(errs, fmt.Errorf("无效的 transport.protocol，可选值为 %v", SupportedTransportProtocols))
	}

	for _, f := range c.IncludeConfigFiles {
		absDir, err := filepath.Abs(filepath.Dir(f))
		if err != nil {
			errs = AppendError(errs, fmt.Errorf("include: 解析 %s 目录失败: %v", f, err))
			continue
		}
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			errs = AppendError(errs, fmt.Errorf("include: %s 目录不存在", f))
		}
	}
	return warnings, errs
}

func ValidateAllClientConfig(c *v1.ClientCommonConfig, proxyCfgs []v1.ProxyConfigurer, visitorCfgs []v1.VisitorConfigurer) (Warning, error) {
	var warnings Warning
	if c != nil {
		warning, err := ValidateClientCommonConfig(c)
		warnings = AppendError(warnings, warning)
		if err != nil {
			return warnings, err
		}
	}

	for _, c := range proxyCfgs {
		if err := ValidateProxyConfigurerForClient(c); err != nil {
			return warnings, fmt.Errorf("proxy %s: %v", c.GetBaseConfig().Name, err)
		}
	}

	for _, c := range visitorCfgs {
		if err := ValidateVisitorConfigurer(c); err != nil {
			return warnings, fmt.Errorf("visitor %s: %v", c.GetBaseConfig().Name, err)
		}
	}
	return warnings, nil
}
