export type Severity = "" | "DEBUG" | "INFO" | "WARNING" | "ERROR";
export type SortOrder = "asc" | "desc";
export type Theme = "dark" | "light";
export type AppView = "explorer" | "alerts" | "nodes";

export type NodeStatus = "registering" | "online" | "offline" | "revoked" | string;

export interface NodeInfo {
	id: string;
	name: string;
	fingerprint: string;
	hostname: string;
	os: string;
	architecture: string;
	agentVersion?: string;
	protocolVersion: number;
	connectedAt?: string;
	lastSeenAt?: string;
	status: NodeStatus;
}

export interface ContainerInfo {
	id: string;
	name: string;
	nodeId?: string;
	nodeName?: string;
	image: string;
	status: string;
	loggingDriver?: string;
	loggingOptions?: Record<string, string>;
	oldestLogAt?: string;
	logCount: number;
	errorCount: number;
	warningCount: number;
}

export interface Resource {
	type: string;
	labels: Record<string, string>;
}

export interface ExplorerEntry {
	insertId: string;
	timestamp: string;
	severity: string;
	logName: string;
	resource: Resource;
	labels: Record<string, string>;
	textPayload?: string;
	jsonPayload?: Record<string, unknown>;
	summary: string;
	stream: string;
}

export interface TimelineBucket {
	start: string;
	end: string;
	total: number;
	severities: Record<string, number>;
}

export interface FieldValue {
	name: string;
	count: number;
	values?: Record<string, number>;
}

export interface FieldGroup {
	name: string;
	fields: FieldValue[];
}

export interface ExplorerResponse {
	entries: ExplorerEntry[];
	containers: ContainerInfo[];
	timeline: TimelineBucket[];
	fields: FieldGroup[];
	total: number;
	nextPageToken?: string;
	generatedAt: string;
	from: string;
	to: string;
	duration: string;
	query: string;
	approximate: boolean;
	logTail: number;
	entryLimit: number;
	truncated: boolean;
	errors?: string[];
}

export interface AlertRule {
	id: string;
	name: string;
	query: string;
	severity?: string;
	labels?: Record<string, string>;
	runbookUrl?: string;
	sampleMode: "off" | "summary" | "full" | string;
	threshold: number;
	windowSeconds: number;
	cooldownSeconds: number;
	enabled: boolean;
	webhookConfigured: boolean;
	status: "OK" | "FIRING" | string;
	matchCount: number;
	lastFiredAt?: string;
	firingSince?: string;
	updatedAt: string;
}

export interface AppState {
	view: AppView;
	query: string;
	draftQuery: string;
	searchText: string;
	node: string;
	showQuery: boolean;
	container: string;
	stream: string;
	severity: Severity;
	duration: string;
	live: boolean;
	sort: SortOrder;
	wrap: boolean;
	pageToken: string;
	entries: ExplorerEntry[];
	containers: ContainerInfo[];
	response: ExplorerResponse | null;
	loading: boolean;
	lastUpdated: string;
	tailConnected: boolean;
	tailMessage: string;
	selectedId: string | null;
	expandedFields: string[];
	fieldsHidden: boolean;
	timelineHidden: boolean;
	timeFrom: string;
	timeTo: string;
	theme: Theme;
	navExpanded: boolean;
	errors: {
		status: string;
		explorer: string;
	};
	errorDetails: string[];
	alerts: {
		rules: AlertRule[];
		loading: boolean;
		error: string;
		search: string;
		statusFilter: "all" | "OK" | "FIRING" | "PAUSED";
	};
	nodes: {
		items: NodeInfo[];
		loading: boolean;
		error: string;
	};
}

export interface QuerySuggestion {
	label: string;
	detail: string;
	replacement: string;
	replaceStart: number;
	replaceEnd: number;
	cursorOffset?: number;
}

export interface RenderActions {
	onFieldFilter?: (field: string, value: string) => void;
	onTimelineSelect?: (start: string, end: string) => void;
	onToast?: (message: string) => void;
}
