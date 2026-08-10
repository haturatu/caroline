import { $ } from "../../shared/dom/selectors";
import { formatClockTime, formatNumber, formatTime } from "../../shared/format";
import { state } from "../../app/state";
import { t, tp } from "../../shared/i18n/index";
import { renderFilters } from "../filters/render";
import { renderFields } from "../fields/render";
import { renderTimeline } from "../timeline/render";
import { renderEntries } from "../logs/render";
export { setRenderActions } from "./actions";

export function renderLoading(): void {
	$("#fieldGroups").innerHTML =
		`<div class="panel-loading loading-panel"><span class="spinner"></span><span>${t("fields.loading")}</span></div>`;
	$("#timelineChart").innerHTML =
		`<div class="chart-loading"><span class="spinner"></span><span>${t("timeline.loading")}</span></div>`;
	$("#timelineAxis").innerHTML = "";
	$("#entryList").innerHTML =
		`<div class="empty-state"><span class="spinner"></span><strong>${t("results.loadingLogs")}</strong><p>${t("results.readingLatest")}</p></div>`;
}

export function renderResultsMeta(): void {
	const response = state.response;
	const logTail = response?.logTail || 1000;
	const entryLimit = response?.entryLimit || 50000;
	$("#resultCount").textContent = response
		? tp("results.entries", response.total)
		: t("common.notAvailable");
	const approximateBadge = $("#approximateBadge");
	approximateBadge.toggleAttribute("hidden", !response?.approximate);
	if (response?.approximate) {
		approximateBadge.title = `${t("results.partialTitle", { lines: formatNumber(logTail) })}${response.truncated ? t("results.resultListCapped", { entries: formatNumber(entryLimit) }) : ""}`;
	}
	$("#resultsDescription").textContent = response
		? state.timeFrom
			? `${formatTime(response.from)} – ${formatTime(response.to)}`
			: ""
		: "";
	const refreshStatus = state.loading
		? t("results.refreshing")
		: state.errors.explorer
			? t("results.refreshFailed")
			: state.live
				? state.tailMessage ||
					(state.tailConnected
						? t("results.liveConnected")
						: t("results.connecting"))
				: state.lastUpdated
					? t("results.updated", { time: formatClockTime(state.lastUpdated) })
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
		? t("results.loading")
		: response?.nextPageToken
			? t("results.loadMore")
			: t("results.noMoreResults");
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
		? t("results.streaming")
		: t("results.paused");
	$("#streamButton").setAttribute(
		"title",
		state.live ? t("results.liveStream") : t("results.liveStreamPaused"),
	);
	$("#fieldsToggle").textContent = state.fieldsHidden ? "›" : "‹";
	$("#fieldsToggle").setAttribute(
		"aria-label",
		state.fieldsHidden ? t("fields.show") : t("fields.hide"),
	);
	$("#fieldsToggle").setAttribute(
		"title",
		state.fieldsHidden ? t("fields.show") : t("fields.hide"),
	);
	const timelineToggle = $("#timelineToggle");
	timelineToggle.textContent = state.timelineHidden ? "⌄" : "⌃";
	timelineToggle.setAttribute(
		"aria-label",
		state.timelineHidden ? t("timeline.expand") : t("timeline.collapse"),
	);
	timelineToggle.setAttribute(
		"title",
		state.timelineHidden ? t("timeline.expand") : t("timeline.collapse"),
	);
}
