export type Severity = "" | "DEBUG" | "INFO" | "WARNING" | "ERROR";
export type SortOrder = "asc" | "desc";
export type Theme = "dark" | "light";

export interface ContainerInfo {
	id: string;
	name: string;
	image: string;
	status: string;
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

export interface AppState {
	query: string;
	draftQuery: string;
	searchText: string;
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
	selectedId: string | null;
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
