package agentproto

import "sort"

const (
	ProtocolVersion = 2
	ProtocolHeader  = "Caroline-Agent-Protocol"
	MaxBatchEntries = 500
	MaxBatchBytes   = 1 << 20
)

var SupportedCapabilities = []string{"logs", "heartbeat", "control", "gzip", "zstd"}

func NegotiateCapabilities(requested []string) []string {
	allowed := make(map[string]bool, len(SupportedCapabilities))
	for _, capability := range SupportedCapabilities {
		allowed[capability] = true
	}
	result := make([]string, 0, len(requested))
	seen := make(map[string]bool, len(requested))
	for _, capability := range requested {
		if allowed[capability] && !seen[capability] {
			result = append(result, capability)
			seen[capability] = true
		}
	}
	sort.Strings(result)
	return result
}
