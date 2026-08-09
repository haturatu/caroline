import { state } from "./state.js";
import type { ExplorerEntry, ExplorerResponse, Severity } from "./types.js";

const supportedDurations = ["5m", "15m", "1h", "6h", "24h", "7d"];

function quoteQueryValue(value: string): string {
	return `"${value.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}

/** Basic controls and the advanced editor are sent as one visible query. */
export function buildBasicQuery(): string[] {
	const parts: string[] = [];
	if (state.container) {
		const container = state.containers.find(
			(item) => item.id === state.container,
		);
		if (container) {
			parts.push(
				`resource.labels.container_name = ${quoteQueryValue(container.name)}`,
			);
		}
	}
	if (state.stream) parts.push(`stream = ${quoteQueryValue(state.stream)}`);
	if (state.severity) parts.push(`severity = ${state.severity}`);
	if (state.searchText)
		parts.push(`SEARCH(${quoteQueryValue(state.searchText.trim())})`);
	return parts;
}

export function buildExplorerQuery(): string {
	return [...buildBasicQuery(), state.query.trim()]
		.filter(Boolean)
		.join(" AND ");
}

export async function getJSON<T>(url: string): Promise<T> {
	const response = await fetch(url, {
		headers: { Accept: "application/json" },
	});
	const payload = (await response.json().catch(() => ({}))) as {
		error?: string;
	};
	if (!response.ok)
		throw new Error(payload.error || `Request failed (${response.status})`);
	return payload as T;
}

export function hydrateURL(): void {
	const params = new URL(window.location.href).searchParams;
	const severity = params.get("severity") || "";
	const duration = params.get("duration") || "5m";
	state.query = params.get("q") || "";
	state.draftQuery = state.query;
	state.searchText = params.get("search") || "";
	state.showQuery = params.get("advanced") === "1" || Boolean(state.query);
	state.container = params.get("container") || "";
	state.stream = params.get("stream") || "";
	state.severity = ["", "DEBUG", "INFO", "WARNING", "ERROR"].includes(severity)
		? (severity as Severity)
		: "";
	state.duration = supportedDurations.includes(duration) ? duration : "5m";
	state.timeFrom = params.get("from") || "";
	state.timeTo = params.get("to") || "";
	state.live = params.get("live") !== "0";
	state.sort = params.get("sort") === "asc" ? "asc" : "desc";
	state.wrap = params.get("wrap") === "1";
	if (params.has("fields")) state.fieldsHidden = params.get("fields") !== "1";
	if (params.has("timeline"))
		state.timelineHidden = params.get("timeline") === "0";
	state.pageToken = params.get("pageToken") || "";
}

export function syncURL(): void {
	const url = new URL(window.location.href);
	const values: Record<string, string> = {
		q: state.query,
		search: state.searchText,
		container: state.container,
		stream: state.stream,
		severity: state.severity,
		duration: state.duration,
		live: state.live ? "1" : "0",
		sort: state.sort,
		from: state.timeFrom,
		to: state.timeTo,
		advanced: state.showQuery ? "1" : "0",
		wrap: state.wrap ? "1" : "0",
		fields: state.fieldsHidden ? "0" : "1",
		timeline: state.timelineHidden ? "0" : "1",
		pageToken: state.pageToken,
	};
	Object.entries(values).forEach(([key, value]) => {
		if (value) url.searchParams.set(key, value);
		else url.searchParams.delete(key);
	});
	window.history.replaceState(null, "", `${url.pathname}${url.search}`);
}

export function buildExplorerURL(): string {
	const params = new URLSearchParams({
		duration: state.duration,
		limit: "100",
		sort: state.sort,
	});
	const query = buildExplorerQuery();
	if (query) params.set("q", query);
	if (state.container) params.set("containers", state.container);
	if (state.timeFrom) params.set("from", state.timeFrom);
	if (state.timeTo) params.set("to", state.timeTo);
	if (state.pageToken) params.set("pageToken", state.pageToken);
	return `/api/explorer?${params.toString()}`;
}

export function buildTailURL(since: string): string {
	const params = new URLSearchParams({ since });
	const query = buildExplorerQuery();
	if (query) params.set("q", query);
	if (state.container) params.set("containers", state.container);
	if (state.stream) params.set("stream", state.stream);
	if (state.severity) params.set("severity", state.severity);
	return `/api/tail?${params.toString()}`;
}

export type StatusResponse = {
	connected: boolean;
	dockerVersion?: string;
};

export async function fetchExplorer(): Promise<ExplorerResponse> {
	return getJSON<ExplorerResponse>(buildExplorerURL());
}

export async function fetchStatus(): Promise<StatusResponse> {
	return getJSON<StatusResponse>("/api/status");
}

export type TailHandlers = {
	onOpen: () => void;
	onEntry: (entry: ExplorerEntry) => void;
	onWarning: (message: string) => void;
	onServerError: (message: string) => void;
	onDisconnect: () => void;
};

function eventPayload<T>(event: Event): T | null {
	const data = (event as MessageEvent<string>).data;
	if (!data) return null;
	try {
		return JSON.parse(data) as T;
	} catch {
		return null;
	}
}

export function openTail(since: string, handlers: TailHandlers): EventSource {
	const source = new EventSource(buildTailURL(since));
	source.addEventListener("open", handlers.onOpen);
	source.addEventListener("log", (event) => {
		const entry = eventPayload<ExplorerEntry>(event);
		if (entry) handlers.onEntry(entry);
	});
	source.addEventListener("warning", (event) => {
		const payload = eventPayload<{ message?: string }>(event);
		if (payload?.message) handlers.onWarning(payload.message);
	});
	source.addEventListener("error", (event) => {
		const payload = eventPayload<{ message?: string }>(event);
		if (payload?.message) handlers.onServerError(payload.message);
		else handlers.onDisconnect();
	});
	return source;
}
