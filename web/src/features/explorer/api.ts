import { state } from "../../app/state";
import { getJSON } from "../../shared/api/http";
import { buildExplorerQuery } from "./query";
import type { ExplorerResponse } from "../../shared/types";

export const minTimelineBuckets = 24;
export const maxTimelineBuckets = 96;
let timelineBuckets = minTimelineBuckets;

export function setTimelineBuckets(value: number): void {
	timelineBuckets = Math.min(
		maxTimelineBuckets,
		Math.max(minTimelineBuckets, Math.round(value)),
	);
}

export function buildExplorerURL(): string {
	const params = new URLSearchParams({
		duration: state.duration,
		limit: "100",
		sort: state.sort,
		timelineBuckets: String(timelineBuckets),
	});
	const query = buildExplorerQuery();
	if (query) params.set("q", query);
	if (state.container) params.set("containers", state.container);
	if (state.timeFrom) params.set("from", state.timeFrom);
	if (state.timeTo) params.set("to", state.timeTo);
	if (state.pageToken) params.set("pageToken", state.pageToken);
	return `/api/explorer?${params.toString()}`;
}

export type StatusResponse = {
	connected: boolean;
	dockerVersion?: string;
};

export function fetchExplorer(): Promise<ExplorerResponse> {
	return getJSON<ExplorerResponse>(buildExplorerURL());
}

export function fetchStatus(): Promise<StatusResponse> {
	return getJSON<StatusResponse>("/api/status");
}
