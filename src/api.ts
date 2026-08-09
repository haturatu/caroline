import { state } from "./state.js";
import type { ExplorerResponse, Severity } from "./types.js";

const supportedDurations = ["5m", "15m", "1h", "6h", "24h", "7d"];

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
	state.showQuery = Boolean(state.query);
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
	const queryParts: string[] = [];
	if (state.searchText)
		queryParts.push('SEARCH("' + state.searchText.replace(/"/g, '\\"') + '")');
	if (state.query) queryParts.push(state.query);
	if (queryParts.length) params.set("q", queryParts.join(" AND "));
	if (state.container) params.set("containers", state.container);
	if (state.stream) params.set("stream", state.stream);
	if (state.severity) params.set("severity", state.severity);
	if (state.timeFrom) params.set("from", state.timeFrom);
	if (state.timeTo) params.set("to", state.timeTo);
	if (state.pageToken) params.set("pageToken", state.pageToken);
	return `/api/explorer?${params.toString()}`;
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
