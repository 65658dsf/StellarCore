package proxy

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
	"github.com/65658dsf/StellarCore/pkg/util/protoinspect"
	"github.com/65658dsf/StellarCore/pkg/util/xlog"
	"github.com/65658dsf/StellarCore/server/metrics"
)

type detectionAction string

const (
	detectionActionBlock      detectionAction = "block"
	detectionActionAllow      detectionAction = "allow"
	detectionActionSuspicious detectionAction = "suspicious"
)

func shouldSkipTrafficInspection(serverCfg *v1.ServerConfig, proxyName string, runID string) bool {
	if serverCfg == nil {
		return true
	}
	if slices.Contains(serverCfg.TrafficMonitor.WhitelistProxies, proxyName) {
		return true
	}
	return runID != "" && slices.Contains(serverCfg.TrafficMonitor.WhitelistRunIDs, runID)
}

func shouldBlockVPNDetection(serverCfg *v1.ServerConfig, result protoinspect.DetectionResult) bool {
	if serverCfg == nil || serverCfg.TrafficMonitor.BlockVPN == nil || !*serverCfg.TrafficMonitor.BlockVPN {
		return false
	}
	if !result.Matched || result.Confidence < 100 || !protoinspect.IsKnownVPNProtocol(result.Protocol) {
		return false
	}

	if len(serverCfg.TrafficMonitor.VPNProtocols) == 0 {
		return true
	}

	protocol := v1.NormalizeVPNProtocol(result.Protocol)
	return slices.Contains(serverCfg.TrafficMonitor.VPNProtocols, protocol)
}

func observeTrafficDetection(
	ctx context.Context,
	action detectionAction,
	transport string,
	proxyName string,
	runID string,
	peekBytes int,
	result protoinspect.DetectionResult,
) {
	if !result.Matched {
		return
	}

	reason := protoinspect.JoinReasonCodes(result.ReasonCodes)
	switch action {
	case detectionActionBlock:
		metrics.Server.AddDetectionBlock(transport, result.Protocol, reason)
	case detectionActionAllow:
		metrics.Server.AddDetectionAllow(transport, result.Protocol, reason)
	case detectionActionSuspicious:
		metrics.Server.AddDetectionSuspicious(transport, result.Protocol, reason)
	}

	levelLogger := xlog.FromContextSafe(ctx)
	message := fmt.Sprintf(
		"[DetectAction=%s] transport=%s protocol=%s confidence=%d reasons=%s peekBytes=%d proxy=%s runID=%s features=%s",
		action,
		transport,
		result.Protocol,
		result.Confidence,
		reason,
		peekBytes,
		proxyName,
		runID,
		formatDetectionFeatures(result.Features),
	)
	switch action {
	case detectionActionBlock, detectionActionSuspicious:
		levelLogger.Warnf(message)
	default:
		levelLogger.Infof(message)
	}
}

func formatDetectionFeatures(features map[string]string) string {
	if len(features) == 0 {
		return "none"
	}
	keys := slices.Sorted(maps.Keys(features))
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+features[key])
	}
	return strings.Join(parts, ",")
}
