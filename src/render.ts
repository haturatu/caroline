import { $$, $, escapeHTML } from "./dom.js";
import {
	formatNumber,
	formatClockTime,
	formatTime,
	formatTimelineTick,
	severityClass,
} from "./format.js";
import { buildBasicQuery } from "./api.js";
import { state } from "./state.js";
import type { ExplorerEntry, RenderActions, TimelineBucket } from "./types.js";

let actions: RenderActions = {};
let timelineDragStartX: number | null = null;
let timelineDragPointerId: number | null = null;
let timelinePressBar: HTMLButtonElement | null = null;
let suppressTimelineClick = false;

export function setRenderActions(nextActions: RenderActions): void {
	actions = nextActions;
}

export function renderLoading(): void {
	$("#fieldGroups").innerHTML =
		'<div class="panel-loading loading-panel"><span class="spinner"></span><span>Loading Fields…</span></div>';
	$("#timelineChart").innerHTML =
		'<div class="chart-loading"><span class="spinner"></span><span>Loading Timeline…</span></div>';
	$("#timelineAxis").innerHTML = "";
	$("#entryList").innerHTML =
		'<div class="empty-state"><span class="spinner"></span><strong>Loading Logs…</strong><p>Reading the latest entries from Docker Engine.</p></div>';
}

export function renderFilters(): void {
	const select = $("#containerFilter") as HTMLSelectElement;
	const options = [
		'<option value="">All containers</option>',
		...state.containers.map(
			(container) =>
				`<option value="${escapeHTML(container.id)}">${escapeHTML(container.name)}</option>`,
		),
	];
	select.innerHTML = options.join("");
	select.value = state.container;
	($("#streamFilter") as HTMLSelectElement).value = state.stream;
	($("#severityFilter") as HTMLSelectElement).value = state.severity;
	($("#rangeFilter") as HTMLSelectElement).value = state.timeFrom
		? "custom"
		: state.duration;
	const customRange = $("#customRangeEditor");
	customRange.toggleAttribute("hidden", !state.timeFrom || !state.timeTo);
	($("#customFromInput") as HTMLInputElement).value = state.timeFrom
		? toDateTimeLocal(state.timeFrom)
		: "";
	($("#customToInput") as HTMLInputElement).value = state.timeTo
		? toDateTimeLocal(state.timeTo)
		: "";
	($("#sortFilter") as HTMLSelectElement).value = state.sort;
	($("#queryInput") as HTMLTextAreaElement).value = state.draftQuery;
	($("#searchAllFields") as HTMLInputElement).value = state.searchText;
	$("#combinedQueryPreview").textContent =
		[...buildBasicQuery(), state.draftQuery.trim()]
			.filter(Boolean)
			.join(" AND ") || "All Logs";
	$("#queryEditor").toggleAttribute("hidden", !state.showQuery);
	$("#showQueryButton").textContent = state.showQuery
		? "Hide Query"
		: "Show Query";
}

function toDateTimeLocal(value: string): string {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return "";
	const pad = (part: number) => String(part).padStart(2, "0");
	return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

export function renderFields(): void {
	const target = $("#fieldGroups");
	if (state.loading && !state.response) {
		target.innerHTML =
			'<div class="panel-loading loading-panel"><span class="spinner"></span><span>Loading Fields…</span></div>';
		return;
	}
	if (!state.response?.fields.length) {
		target.innerHTML =
			'<div class="panel-loading">No fields were found in the current result set.</div>';
		return;
	}
	const availableFields = new Set(
		state.response.fields.flatMap((group) =>
			group.fields.map((field) => field.name),
		),
	);
	state.expandedFields = state.expandedFields.filter((field) =>
		availableFields.has(field),
	);
	const expandedFields = new Set(state.expandedFields);
	target.innerHTML = state.response.fields
		.map((group) => {
			const fields = group.fields
				.map((field) => {
					const isExpanded = expandedFields.has(field.name);
					const values = Object.entries(field.values || {})
						.slice(0, 5)
						.map(
							([value, count]) =>
								`<button class="field-value" type="button" data-field="${escapeHTML(field.name)}" data-value="${escapeHTML(value)}">${escapeHTML(value)} <span>(${formatNumber(count)})</span></button>`,
						)
						.join("");
					return `<button class="field-row${isExpanded ? " expanded" : ""}" type="button" aria-expanded="${isExpanded}" data-field="${escapeHTML(field.name)}"><span class="field-name" title="${escapeHTML(field.name)}">${escapeHTML(field.name)}</span><span class="field-count">${formatNumber(field.count)}</span></button><div class="field-values">${values}</div>`;
				})
				.join("");
			return `<section class="field-group"><div class="field-group-title">${escapeHTML(group.name)}</div>${fields}</section>`;
		})
		.join("");
	$$<HTMLButtonElement>(".field-row").forEach((button) =>
		button.addEventListener("click", () => {
			const expanded = !button.classList.contains("expanded");
			const field = button.dataset.field;
			button.classList.toggle("expanded", expanded);
			button.setAttribute("aria-expanded", String(expanded));
			if (field) {
				const fields = new Set(state.expandedFields);
				if (expanded) fields.add(field);
				else fields.delete(field);
				state.expandedFields = [...fields];
			}
		}),
	);
	$$<HTMLButtonElement>(".field-value").forEach((button) =>
		button.addEventListener("click", () => {
			const field = button.getAttribute("data-field");
			const value = button.getAttribute("data-value");
			if (field && value) actions.onFieldFilter?.(field, value);
		}),
	);
}

function timelineSegment(
	bucket: TimelineBucket,
	name: string,
	className: string,
): string {
	return `<span class="timeline-segment ${className}" style="height:${Math.max(0, ((bucket.severities[name] || 0) / Math.max(1, bucket.total)) * 100)}%"></span>`;
}

function timelineBucketLabel(bucket: TimelineBucket): string {
	const start = Date.parse(bucket.start);
	const end = Date.parse(bucket.end);
	const midpoint =
		Number.isNaN(start) || Number.isNaN(end)
			? bucket.start
			: new Date((start + end) / 2).toISOString();
	return `${formatTimelineTick(midpoint)} · ${formatNumber(bucket.total)} entries`;
}

function timelineBucketAriaLabel(bucket: TimelineBucket): string {
	const severityLabels: Record<string, string> = {
		ERROR: "errors",
		WARNING: "warnings",
		INFO: "info",
		DEBUG: "debug",
	};
	const breakdown = ["ERROR", "WARNING", "INFO", "DEBUG"]
		.filter((severity) => (bucket.severities[severity] || 0) > 0)
		.map(
			(severity) =>
				`${formatNumber(bucket.severities[severity] || 0)} ${severityLabels[severity]}`,
		)
		.join(", ");
	return `${formatTime(bucket.start)} to ${formatTime(bucket.end)}. ${formatNumber(bucket.total)} entries${breakdown ? `: ${breakdown}` : ": no severity breakdown"}. Select this interval.`;
}

function renderTimelineLegend(): void {
	const legend = $("#timelineLegend");
	legend.innerHTML = [
		["error", "Errors"],
		["warning", "Warnings"],
		["info", "Info"],
		["debug", "Debug"],
	]
		.map(
			([className, label]) =>
				`<span class="timeline-legend-item"><span class="timeline-legend-swatch ${className}" aria-hidden="true"></span><span>${label}</span></span>`,
		)
		.join("");
}

function timelineSelection(response: { from: string; to: string }): {
	left: number;
	right: number;
} {
	const from = Date.parse(response.from);
	const to = Date.parse(response.to);
	const selectedFrom = state.timeFrom ? Date.parse(state.timeFrom) : from;
	const selectedTo = state.timeTo ? Date.parse(state.timeTo) : to;
	if (
		Number.isNaN(from) ||
		Number.isNaN(to) ||
		to <= from ||
		Number.isNaN(selectedFrom) ||
		Number.isNaN(selectedTo)
	)
		return { left: 0, right: 0 };
	const clamp = (value: number) => Math.min(100, Math.max(0, value));
	const left = clamp(((selectedFrom - from) / (to - from)) * 100);
	const end = clamp(((selectedTo - from) / (to - from)) * 100);
	return { left, right: 100 - Math.max(left, end) };
}

function timelineXAtClientX(chart: HTMLElement, clientX: number): number {
	const bounds = chart.getBoundingClientRect();
	return Math.min(bounds.width, Math.max(0, clientX - bounds.left));
}

function timelineTimeAtX(chart: HTMLElement, x: number): number | null {
	const response = state.response;
	if (!response) return null;
	const from = Date.parse(response.from);
	const to = Date.parse(response.to);
	const width = chart.getBoundingClientRect().width;
	if (!Number.isFinite(from) || !Number.isFinite(to) || to <= from || width <= 0)
		return null;
	return from + (x / width) * (to - from);
}

function updateTimelineDrag(chart: HTMLElement, currentX: number): void {
	if (timelineDragStartX === null) return;
	const left = Math.min(timelineDragStartX, currentX);
	const right = Math.max(timelineDragStartX, currentX);
	const selection = chart.querySelector<HTMLElement>(
		".timeline-drag-selection",
	);
	const label = chart.querySelector<HTMLElement>(".timeline-drag-label");
	if (selection) {
		selection.style.left = `${left}px`;
		selection.style.width = `${right - left}px`;
	}
	const bounds = chart.getBoundingClientRect();
	if (label && bounds.width > 0) {
		const from = timelineTimeAtX(chart, left);
		const to = timelineTimeAtX(chart, right);
		if (from !== null && to !== null) {
			label.textContent = `${formatTimelineTick(new Date(from).toISOString())} – ${formatTimelineTick(new Date(to).toISOString())}`;
			label.style.left = `${((left + right) / 2 / bounds.width) * 100}%`;
		}
	}
}

function cleanupTimelineDrag(chart: HTMLElement): void {
	if (
		timelineDragPointerId !== null &&
		chart.hasPointerCapture(timelineDragPointerId)
	) {
		chart.releasePointerCapture(timelineDragPointerId);
	}
	timelineDragStartX = null;
	timelineDragPointerId = null;
	timelinePressBar = null;
	chart.classList.remove("selecting");
	const selection = chart.querySelector<HTMLElement>(
		".timeline-drag-selection",
	);
	if (selection) {
		selection.style.left = "0px";
		selection.style.width = "0px";
	}
	const label = chart.querySelector<HTMLElement>(".timeline-drag-label");
	if (label) label.textContent = "";
}

export function renderTimeline(): void {
	const response = state.response;
	const chart = $("#timelineChart");
	const axis = $("#timelineAxis");
	if (state.loading && !response) {
		chart.innerHTML =
			'<div class="chart-loading"><span class="spinner"></span><span>Loading Timeline…</span></div>';
		axis.innerHTML = "";
		$("#timelineLegend").innerHTML = "";
		return;
	}
	if (!response?.timeline.length) {
		chart.innerHTML =
			'<div class="chart-loading">No timeline data for this query.</div>';
		axis.innerHTML = "";
		$("#timelineLegend").innerHTML = "";
		return;
	}
	const maximum = Math.max(
		1,
		...response.timeline.map((bucket) => bucket.total),
	);
	const bars = response.timeline
		.map((bucket, index) => {
			if (!bucket.total) return "";
			const height = Math.max(4, Math.round((bucket.total / maximum) * 68));
			const position = ((index + 0.5) / response.timeline.length) * 100;
			return `<button class="timeline-bar" type="button" data-index="${index}" data-start="${escapeHTML(bucket.start)}" data-end="${escapeHTML(bucket.end)}" style="--bar-position:${position}%;--bar-height:${height}px" aria-label="${escapeHTML(timelineBucketAriaLabel(bucket))}"><span class="timeline-bar-inner">${timelineSegment(bucket, "ERROR", "error")}${timelineSegment(bucket, "WARNING", "warning")}${timelineSegment(bucket, "INFO", "info")}${timelineSegment(bucket, "DEBUG", "debug")}</span></button>`;
		})
		.join("");
	const selection = timelineSelection(response);
	renderTimelineLegend();
	const initialBucket =
		response.timeline[Math.floor(response.timeline.length / 2)] ||
		response.timeline[0];
	chart.innerHTML = `<div class="timeline-grid" aria-hidden="true"><span></span><span></span><span></span><span></span></div><div class="timeline-selection" style="left:${selection.left}%;right:${selection.right}%" aria-hidden="true"></div><div class="timeline-drag-selection" aria-hidden="true"></div><span class="timeline-drag-label" aria-hidden="true"></span><div class="timeline-bars">${bars}</div><div class="timeline-baseline" aria-hidden="true"></div><div class="timeline-cursor" style="left:50%" aria-hidden="true"><span class="timeline-cursor-badge" title="${escapeHTML(timelineBucketLabel(initialBucket))}">${escapeHTML(timelineBucketLabel(initialBucket))}</span></div>`;
	const tickCount = 7;
	axis.innerHTML = Array.from({ length: tickCount }, (_, index) => {
		const bucketIndex = Math.min(
			response.timeline.length - 1,
			Math.round((index * (response.timeline.length - 1)) / (tickCount - 1)),
		);
		const value =
			index === 0
				? response.from
				: index === tickCount - 1
					? response.to
					: response.timeline[bucketIndex].start;
		return `<span>${formatTimelineTick(value)}</span>`;
	}).join("");
	if (!chart.dataset.cursorBound) {
		chart.addEventListener("mousemove", (event: MouseEvent) => {
			const bounds = chart.getBoundingClientRect();
			const position = Math.min(
				1,
				Math.max(0, (event.clientX - bounds.left) / bounds.width),
			);
			const cursor = chart.querySelector<HTMLElement>(".timeline-cursor");
			const timeline = state.response?.timeline || [];
			if (cursor) cursor.style.left = `${position * 100}%`;
			if (timeline.length) {
				const bucket =
					timeline[
						Math.min(
							timeline.length - 1,
							Math.floor(position * timeline.length),
						)
					];
				const label = timelineBucketLabel(bucket);
				const badge = chart.querySelector<HTMLElement>(
					".timeline-cursor-badge",
				);
				if (badge) {
					badge.textContent = label;
					badge.title = label;
				}
			}
		});
		chart.addEventListener("mouseleave", () => {
			if (timelineDragStartX !== null) return;
			const cursor = chart.querySelector<HTMLElement>(".timeline-cursor");
			const timeline = state.response?.timeline || [];
			if (cursor) cursor.style.left = "50%";
			if (timeline.length) {
				const badge = chart.querySelector<HTMLElement>(
					".timeline-cursor-badge",
				);
				if (badge) {
					const label = timelineBucketLabel(
						timeline[Math.floor(timeline.length / 2)],
					);
					badge.textContent = label;
					badge.title = label;
				}
			}
		});
		chart.dataset.cursorBound = "true";
	}
	if (!chart.dataset.dragBound) {
		chart.addEventListener("pointerdown", (event: PointerEvent) => {
			if (event.button !== 0 || !state.response?.timeline.length) return;
			timelinePressBar =
				event.target instanceof Element
					? event.target.closest<HTMLButtonElement>(".timeline-bar")
					: null;
			timelineDragStartX = timelineXAtClientX(chart, event.clientX);
			timelineDragPointerId = event.pointerId;
			chart.setPointerCapture(event.pointerId);
			chart.classList.add("selecting");
			updateTimelineDrag(chart, timelineDragStartX);
		});
		chart.addEventListener("pointermove", (event: PointerEvent) => {
			if (
				timelineDragStartX === null ||
				timelineDragPointerId !== event.pointerId
			)
				return;
			updateTimelineDrag(chart, timelineXAtClientX(chart, event.clientX));
		});
		chart.addEventListener("pointerup", (event: PointerEvent) => {
			if (
				timelineDragStartX === null ||
				timelineDragPointerId !== event.pointerId
			)
				return;
			const startX = timelineDragStartX;
			const endX = timelineXAtClientX(chart, event.clientX);
			const distance = Math.abs(endX - startX);
			const pressedBar = timelinePressBar;
			const from = timelineTimeAtX(chart, Math.min(startX, endX));
			const to = timelineTimeAtX(chart, Math.max(startX, endX));
			cleanupTimelineDrag(chart);
			if (distance < 4) {
				if (pressedBar) {
					suppressTimelineClick = true;
					window.setTimeout(() => {
						suppressTimelineClick = false;
					}, 50);
					const start = pressedBar.getAttribute("data-start");
					const end = pressedBar.getAttribute("data-end");
					if (start && end) actions.onTimelineSelect?.(start, end);
				}
				return;
			}
			if (from === null || to === null) return;
			suppressTimelineClick = true;
			window.setTimeout(() => {
				suppressTimelineClick = false;
			}, 50);
			actions.onTimelineSelect?.(
				new Date(from).toISOString(),
				new Date(to).toISOString(),
			);
		});
		chart.addEventListener("pointercancel", () =>
			cleanupTimelineDrag(chart),
		);
		chart.dataset.dragBound = "true";
	}
	const setTimelineBadge = (bar: HTMLButtonElement): void => {
		const timeline = state.response?.timeline || [];
		const index = Number(bar.dataset.index || 0);
		const bucket = timeline[index];
		if (!bucket) return;
		const cursor = chart.querySelector<HTMLElement>(".timeline-cursor");
		const badge = chart.querySelector<HTMLElement>(".timeline-cursor-badge");
		if (cursor)
			cursor.style.left = `${((index + 0.5) / timeline.length) * 100}%`;
		if (badge) {
			badge.textContent = timelineBucketLabel(bucket);
			badge.title = timelineBucketLabel(bucket);
		}
	};
	$$<HTMLButtonElement>(".timeline-bar").forEach((bar) => {
		bar.addEventListener("focus", () => setTimelineBadge(bar));
		bar.addEventListener("click", () => {
			if (suppressTimelineClick) return;
			const start = bar.getAttribute("data-start");
			const end = bar.getAttribute("data-end");
			if (start && end) actions.onTimelineSelect?.(start, end);
		});
	});
}

export function entrySummary(entry: ExplorerEntry): string {
	return (
		entry.summary ||
		entry.textPayload ||
		(entry.jsonPayload ? JSON.stringify(entry.jsonPayload) : "")
	);
}

export function renderEntries(): void {
	const list = $("#entryList");
	if (state.loading && !state.response) {
		list.innerHTML =
			'<div class="empty-state"><span class="spinner"></span><strong>Loading Logs…</strong><p>Reading the latest entries from Docker Engine.</p></div>';
		return;
	}
	if (!state.entries.length) {
		list.innerHTML = state.response
			? '<div class="empty-state"><span class="empty-state-icon" aria-hidden="true">⌕</span><strong>No Logs Match These Filters</strong><p>Try a broader time range or a less specific query.</p><div class="empty-state-actions"><button class="text-button" type="button" data-empty-action="reset">Reset Filters</button><button class="text-button" type="button" data-empty-action="hour">Last 1 Hour</button></div></div>'
			: '<div class="empty-state"><span class="empty-state-icon" aria-hidden="true">⌕</span><strong>Run a Query</strong><p>Your matching log entries will appear here.</p></div>';
		return;
	}
	list.classList.toggle("wrap-lines", state.wrap);
	list.innerHTML = state.entries
		.map(
			(entry) =>
				`<button class="entry-row ${state.selectedId === entry.insertId ? "selected" : ""}" type="button" data-entry-id="${escapeHTML(entry.insertId)}" aria-label="${escapeHTML(`${entry.severity} log from ${entry.resource.labels.container_name}: ${entrySummary(entry)}`)}"><span class="entry-time">${escapeHTML(formatTime(entry.timestamp))}</span><span class="entry-severity ${severityClass(entry.severity)}">${escapeHTML(entry.severity)}</span><span class="entry-resource"><span class="resource-name">${escapeHTML(entry.resource.labels.container_name || "Unknown Container")}</span><span class="resource-meta">${escapeHTML(entry.stream)} · ${escapeHTML(entry.resource.type)}</span></span><span class="entry-summary" title="${escapeHTML(entrySummary(entry))}">${escapeHTML(entrySummary(entry))}</span><span class="entry-open"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 5 7 7-7 7"/></svg></span></button>`,
		)
		.join("");
}

export function renderDetail(): void {
	const entry = state.entries.find(
		(item) => item.insertId === state.selectedId,
	);
	const drawer = $("#detailDrawer");
	if (!entry) {
		drawer.setAttribute("hidden", "");
		return;
	}
	drawer.removeAttribute("hidden");
	const payload = entry.jsonPayload || {
		textPayload: entry.textPayload || entry.summary,
	};
	$("#detailTitle").textContent =
		`${entry.severity} · ${entry.resource.labels.container_name || "Container Log"}`;
	$("#detailBody").innerHTML =
		`<section class="detail-section"><span class="detail-label">Timestamp</span><div class="detail-value">${escapeHTML(formatTime(entry.timestamp))}</div></section><section class="detail-section"><span class="detail-label">Summary</span><div class="detail-value">${escapeHTML(entrySummary(entry))}</div></section><section class="detail-section"><span class="detail-label">Payload</span><pre class="detail-code">${escapeHTML(JSON.stringify(payload, null, 2))}</pre></section><section class="detail-section"><span class="detail-label">Log Entry Metadata</span><div class="detail-meta"><div class="detail-meta-row"><span>Insert ID</span><strong>${escapeHTML(entry.insertId)}</strong></div><div class="detail-meta-row"><span>Log Name</span><strong>${escapeHTML(entry.logName)}</strong></div><div class="detail-meta-row"><span>Resource Type</span><strong>${escapeHTML(entry.resource.type)}</strong></div><div class="detail-meta-row"><span>Stream</span><strong>${escapeHTML(entry.stream)}</strong></div></div></section><button class="run-button" id="copyEntryButton" type="button">Copy Entry JSON</button>`;
	$("#copyEntryButton").addEventListener("click", () => {
		const copy = navigator.clipboard?.writeText(JSON.stringify(entry, null, 2));
		if (copy) void copy.then(() => actions.onToast?.("Entry JSON copied."));
	});
}

export function renderResultsMeta(): void {
	const response = state.response;
	const logTail = response?.logTail || 1000;
	const entryLimit = response?.entryLimit || 50000;
	$("#resultCount").textContent = response
		? `${formatNumber(response.total)} entries`
		: "—";
	const approximateBadge = $("#approximateBadge");
	approximateBadge.toggleAttribute("hidden", !response?.approximate);
	if (response?.approximate) {
		approximateBadge.title = `Latest ${formatNumber(logTail)} lines per container${response.truncated ? `; result list capped at ${formatNumber(entryLimit)} entries` : ""}`;
	}
	$("#resultsDescription").textContent = response
		? state.timeFrom
			? `${formatTime(response.from)} – ${formatTime(response.to)}`
			: ""
		: "";
	const refreshStatus = state.loading
		? "Refreshing…"
		: state.errors.explorer
			? "Refresh failed · Showing previous results"
			: state.live
				? state.tailMessage ||
					(state.tailConnected
						? "Live stream connected"
						: "Connecting to live log stream…")
				: state.lastUpdated
					? `Updated ${formatClockTime(state.lastUpdated)}`
					: "";
	$("#refreshStatus").textContent = refreshStatus;
	const runButton = $("#runQueryButton") as HTMLButtonElement;
	runButton.dataset.loading = String(state.loading);
	runButton.disabled = state.loading;
	runButton.setAttribute("aria-busy", String(state.loading));
	$("#resultsFooter").textContent = "";
	const next = $("#nextPageButton") as HTMLButtonElement;
	next.toggleAttribute("hidden", !response?.nextPageToken);
	next.disabled = state.loading;
	next.textContent = state.loading
		? "Loading…"
		: response?.nextPageToken
			? "Load More"
			: "No More Results";
}

export function renderAll(): void {
	renderFilters();
	renderFields();
	renderTimeline();
	renderEntries();
	renderResultsMeta();
	$("#fields").classList.toggle("fields-collapsed", state.fieldsHidden);
	$("#timeline").classList.toggle("timeline-collapsed", state.timelineHidden);
	$("#wrapButton").setAttribute("aria-pressed", String(state.wrap));
	$("#streamButton").classList.toggle("paused", !state.live);
	$("#streamButton").setAttribute("aria-pressed", String(state.live));
	$("#streamButtonText").textContent = state.live
		? "Streaming"
		: "Paused";
	$("#streamButton").setAttribute(
		"title",
		state.live ? "Live SSE stream" : "Live stream is paused",
	);
	$("#fieldsToggle").textContent = state.fieldsHidden ? "›" : "‹";
	$("#fieldsToggle").setAttribute(
		"aria-label",
		state.fieldsHidden ? "Show Fields" : "Hide Fields",
	);
	$("#fieldsToggle").setAttribute(
		"title",
		state.fieldsHidden ? "Show Fields" : "Hide Fields",
	);
}
