import { state } from "./state";
import type { Severity } from "../shared/types";

const supportedDurations = ["5m", "15m", "1h", "6h", "24h", "7d"];

export function hydrateURL(): void {
	const params = new URL(window.location.href).searchParams;
	const view = params.get("view");
	state.view = view === "alerts" || view === "nodes" ? view : "explorer";
	const severity = params.get("severity") || "";
	const duration = params.get("duration") || "5m";
	state.query = params.get("q") || "";
	state.draftQuery = state.query;
	state.logName = params.get("logName") || "";
	state.resourceType = params.get("resourceType") || "";
	state.searchText = params.get("search") || "";
	state.node = params.get("node") || "";
	state.showQuery = params.get("advanced") === "1" || Boolean(state.query);
	state.container = params.get("container") || "";
	state.stream = params.get("stream") || "";
	state.severity = [
		"",
		"DEFAULT",
		"DEBUG",
		"INFO",
		"NOTICE",
		"WARNING",
		"ERROR",
		"CRITICAL",
		"ALERT",
		"EMERGENCY",
	].includes(severity)
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
	state.selectedId = params.get("entry");
	state.pageToken = params.get("pageToken") || "";
	const zone = params.get("tz") || "local";
	try {
		new Intl.DateTimeFormat("en", zone === "local" ? {} : { timeZone: zone });
		state.timeZone = zone;
	} catch {
		state.timeZone = "local";
	}
	state.highlight = params.get("highlight") || "";
	state.summaryFields = (params.get("summary") || "")
		.split(",")
		.filter(Boolean)
		.slice(0, 8);
	state.expandedEntries = params.getAll("expand").slice(0, 100);
	state.compact = params.get("compact") === "1";
	state.showSummary = params.get("summaryVisible") !== "0";
	state.showMilliseconds = params.get("precision") !== "seconds";
	const width = Number(params.get("fieldsWidth"));
	const height = Number(params.get("timelineHeight"));
	state.fieldsWidth = width >= 180 && width <= 440 ? width : 235;
	state.timelineHeight = height >= 80 && height <= 360 ? height : 120;
}

export function syncURL(mode: "push" | "replace" = "replace"): void {
	const url = new URL(window.location.href);
	const values: Record<string, string> = {
		view: state.view === "alerts" || state.view === "nodes" ? state.view : "",
		q: state.query,
		search: state.searchText,
		logName: state.logName,
		resourceType: state.resourceType,
		node: state.node,
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
		entry: state.selectedId || "",
		tz: state.timeZone === "local" ? "" : state.timeZone,
		highlight: state.highlight,
		summary: state.summaryFields.join(","),
		compact: state.compact ? "1" : "",
		fieldsWidth: state.fieldsWidth === 235 ? "" : String(state.fieldsWidth),
		timelineHeight:
			state.timelineHeight === 120 ? "" : String(state.timelineHeight),
		precision: state.showMilliseconds ? "" : "seconds",
		summaryVisible: state.showSummary ? "" : "0",
	};
	Object.entries(values).forEach(([key, value]) => {
		if (value) url.searchParams.set(key, value);
		else url.searchParams.delete(key);
	});
	url.searchParams.delete("expand");
	for (const id of state.expandedEntries.slice(0, 100))
		url.searchParams.append("expand", id);
	const next = `${url.pathname}${url.search}`;
	if (
		mode === "push" &&
		next !== `${window.location.pathname}${window.location.search}`
	)
		window.history.pushState(null, "", next);
	else window.history.replaceState(null, "", next);
}
