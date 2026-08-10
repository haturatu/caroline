import { $$, $, escapeHTML } from "../../shared/dom/selectors";
import {
	formatTime,
	formatNumber,
	formatTimelineAxisTick,
	formatTimelineDetail,
} from "../../shared/format";
import { state } from "../../app/state";
import { t, tp } from "../../shared/i18n/index";
import { getRenderActions } from "../explorer/actions";
import type { TimelineBucket } from "../../shared/types";

let timelineDragStartX: number | null = null;
let timelineDragPointerId: number | null = null;
let timelinePressBar: HTMLButtonElement | null = null;
let suppressTimelineClick = false;

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
	return `${formatTimelineDetail(midpoint)} · ${tp("timeline.entriesSummary", bucket.total)}`;
}

function timelineBucketAriaLabel(bucket: TimelineBucket): string {
	const severityLabels: Record<string, string> = {
		ERROR: t("timeline.error"),
		WARNING: t("timeline.warning"),
		INFO: t("timeline.info"),
		DEBUG: t("timeline.debug"),
	};
	const breakdown = ["ERROR", "WARNING", "INFO", "DEBUG"]
		.filter((severity) => (bucket.severities[severity] || 0) > 0)
		.map((severity) =>
			t("timeline.severityCount", {
				count: formatNumber(bucket.severities[severity] || 0),
				severity: severityLabels[severity],
			}),
		)
		.join(", ");
	return t("timeline.bucketAria", {
		from: formatTime(bucket.start),
		to: formatTime(bucket.end),
		entries: tp("timeline.entriesSummary", bucket.total),
		breakdown: breakdown || t("timeline.noSeverityBreakdown"),
	});
}

function timelineDayBoundaries(response: { from: string; to: string }): string {
	const start = Date.parse(response.from);
	const end = Date.parse(response.to);
	const duration = end - start;
	if (
		!Number.isFinite(start) ||
		!Number.isFinite(end) ||
		duration < 3 * 24 * 60 * 60 * 1000
	)
		return "";

	const firstBoundary = new Date(start);
	firstBoundary.setHours(0, 0, 0, 0);
	firstBoundary.setDate(firstBoundary.getDate() + 1);
	const boundaries: string[] = [];
	for (
		let boundary = firstBoundary.getTime();
		boundary < end;
		boundary = new Date(boundary).setDate(new Date(boundary).getDate() + 1)
	) {
		const position = ((boundary - start) / duration) * 100;
		if (position > 0 && position < 100)
			boundaries.push(
				`<span class="timeline-day-boundary" style="left:${position}%"></span>`,
			);
	}
	return boundaries.join("");
}

function renderTimelineLegend(): void {
	const legend = $("#timelineLegend");
	legend.innerHTML = [
		["error", t("timeline.error")],
		["warning", t("timeline.warning")],
		["info", t("timeline.info")],
		["debug", t("timeline.debug")],
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
	if (
		!Number.isFinite(from) ||
		!Number.isFinite(to) ||
		to <= from ||
		width <= 0
	)
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
		const response = state.response;
		if (from !== null && to !== null && response) {
			label.textContent = `${formatTimelineDetail(new Date(from).toISOString())} – ${formatTimelineDetail(new Date(to).toISOString())}`;
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
		chart.innerHTML = `<div class="chart-loading"><span class="spinner"></span><span>${t("timeline.loading")}</span></div>`;
		axis.innerHTML = "";
		$("#timelineLegend").innerHTML = "";
		return;
	}
	if (!response?.timeline.length) {
		chart.innerHTML = `<div class="chart-loading">${t("timeline.noData")}</div>`;
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
			const height = bucket.total
				? Math.max(4, Math.round((bucket.total / maximum) * 68))
				: 2;
			const position = ((index + 0.5) / response.timeline.length) * 100;
			const emptyClass = bucket.total ? "" : " empty";
			return `<button class="timeline-bar${emptyClass}" type="button" data-index="${index}" data-start="${escapeHTML(bucket.start)}" data-end="${escapeHTML(bucket.end)}" style="--bar-position:${position}%;--bar-height:${height}px" aria-label="${escapeHTML(timelineBucketAriaLabel(bucket))}"><span class="timeline-bar-inner">${timelineSegment(bucket, "ERROR", "error")}${timelineSegment(bucket, "WARNING", "warning")}${timelineSegment(bucket, "INFO", "info")}${timelineSegment(bucket, "DEBUG", "debug")}</span></button>`;
		})
		.join("");
	const selection = timelineSelection(response);
	renderTimelineLegend();
	const initialBucket =
		response.timeline[Math.floor(response.timeline.length / 2)] ||
		response.timeline[0];
	const range = { from: response.from, to: response.to };
	const isLongRange =
		Date.parse(response.to) - Date.parse(response.from) >= 12 * 60 * 60 * 1000;
	const dayBoundaries = timelineDayBoundaries(range);
	chart.innerHTML = `<div class="timeline-grid" aria-hidden="true"><span></span><span></span><span></span><span></span></div><div class="timeline-day-boundaries" aria-hidden="true">${dayBoundaries}</div><div class="timeline-selection" style="left:${selection.left}%;right:${selection.right}%" aria-hidden="true"></div><div class="timeline-drag-selection" aria-hidden="true"></div><span class="timeline-drag-label" aria-hidden="true"></span><div class="timeline-bars">${bars}</div><div class="timeline-baseline" aria-hidden="true"></div><div class="timeline-cursor" style="left:50%" aria-hidden="true"><span class="timeline-cursor-badge" title="${escapeHTML(timelineBucketLabel(initialBucket))}">${escapeHTML(timelineBucketLabel(initialBucket))}</span></div>`;
	const tickCount = isLongRange ? 5 : 7;
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
		return `<time datetime="${escapeHTML(value)}" title="${escapeHTML(formatTime(value))}">${formatTimelineAxisTick(value, response.from, response.to)}</time>`;
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
					if (start && end) getRenderActions().onTimelineSelect?.(start, end);
				}
				return;
			}
			if (from === null || to === null) return;
			suppressTimelineClick = true;
			window.setTimeout(() => {
				suppressTimelineClick = false;
			}, 50);
			getRenderActions().onTimelineSelect?.(
				new Date(from).toISOString(),
				new Date(to).toISOString(),
			);
		});
		chart.addEventListener("pointercancel", () => cleanupTimelineDrag(chart));
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
			const label = timelineBucketLabel(bucket);
			badge.textContent = label;
			badge.title = label;
		}
	};
	$$<HTMLButtonElement>(".timeline-bar").forEach((bar) => {
		bar.addEventListener("focus", () => setTimelineBadge(bar));
		bar.addEventListener("click", () => {
			if (suppressTimelineClick) return;
			const start = bar.getAttribute("data-start");
			const end = bar.getAttribute("data-end");
			if (start && end) getRenderActions().onTimelineSelect?.(start, end);
		});
	});
}
