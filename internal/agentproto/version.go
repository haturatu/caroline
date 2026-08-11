package agentproto

const (
	ProtocolVersion = 1
	ProtocolHeader  = "Caroline-Agent-Protocol"
	MaxBatchEntries = 500
	MaxBatchBytes   = 1 << 20
)
