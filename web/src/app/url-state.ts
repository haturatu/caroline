import { state } from "./state";
import type { Severity } from "../shared/types";

const supportedDurations = ["5m", "15m", "1h", "6h", "24h", "7d"];

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
