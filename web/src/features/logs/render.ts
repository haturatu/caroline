import { $, escapeHTML } from "../../shared/dom/selectors";
import { formatTime, severityClass } from "../../shared/format";
import { state } from "../../app/state";
import { getLocale, t } from "../../shared/i18n/index";
import { entryTools, structuredEntry, highlighted } from "./structured";
import { valueAt, displayValue } from "../workspace/model";
import { w } from "../workspace/messages";
import type { ExplorerEntry } from "../../shared/types";

export function entrySummary(entry: ExplorerEntry): string {
	if (state.summaryFields.length) {
		const values = state.summaryFields
			.map((path) => {
				const value = valueAt(entry, path);
				return value === undefined ? "" : `${path}: ${displayValue(value)}`;
			})
			.filter(Boolean);
		if (values.length) return values.join(" · ");
	}
	return (
		entry.summary ||
		entry.textPayload ||
		(entry.jsonPayload ? JSON.stringify(entry.jsonPayload) : "")
	);
}

export function renderEntries(): void {
	const list = $("#entryList");
	if (state.loading && !state.response) {
		list.innerHTML = `<div class="empty-state"><span class="spinner"></span><strong>${t("results.loadingLogs")}</strong><p>${t("results.readingLatest")}</p></div>`;
		return;
	}
	if (!state.entries.length) {
		list.innerHTML = state.response
			? `<div class="empty-state"><span class="empty-state-icon" aria-hidden="true">⌕</span><strong>${t("results.noMatchTitle")}</strong><p>${t("results.noMatchDescription")}</p><div class="empty-state-actions"><button class="text-button" type="button" data-empty-action="reset">${t("results.resetFilters")}</button><button class="text-button" type="button" data-empty-action="hour">${t("results.lastHour")}</button></div></div>`
			: `<div class="empty-state"><span class="empty-state-icon" aria-hidden="true">⌕</span><strong>${t("results.runAQuery")}</strong><p>${t("results.matchingEntries")}</p></div>`;
		return;
	}
	list.classList.toggle("wrap-lines", state.wrap);
	const focused = document.activeElement as HTMLElement | null;
	const inlineSettings = JSON.stringify([
		getLocale(),
		state.highlight,
		state.summaryFields,
	]);
	const previousInline = new Map(
		[...list.querySelectorAll<HTMLElement>("[data-inline-entry]")]
			.filter((node) => node.dataset.renderSettings === inlineSettings)
			.map((node) => [node.dataset.inlineEntry, node]),
	);
	list.innerHTML = state.entries
		.map((entry) => {
			const containerName =
				entry.resource.labels.container_name || t("common.unknownContainer");
			const expanded =
				state.expandedEntries.includes("*") ||
				state.expandedEntries.includes(entry.insertId);
			return `<div class="entry-record"><div class="entry-row-group"><button class="entry-expand icon-button" type="button" data-expand-entry="${escapeHTML(entry.insertId)}" aria-expanded="${expanded}" aria-label="${escapeHTML(w(expanded ? "collapseEntry" : "expandEntry"))}">${expanded ? "▾" : "▸"}</button><button class="entry-row ${state.selectedId === entry.insertId ? "selected" : ""}" type="button" data-entry-id="${escapeHTML(entry.insertId)}" aria-label="${escapeHTML(`${entry.severity} log from ${containerName}: ${entrySummary(entry)}`)}"><time class="entry-time" datetime="${escapeHTML(entry.timestamp)}">${escapeHTML(formatTime(entry.timestamp))}</time><span class="entry-severity ${severityClass(entry.severity)}">${escapeHTML(entry.severity)}</span><span class="entry-resource"><span class="resource-name">${escapeHTML(containerName)}</span><span class="resource-meta">${escapeHTML(entry.stream)} · ${escapeHTML(entry.resource.type)}</span></span><span class="entry-summary" title="${escapeHTML(entrySummary(entry))}">${highlighted(entrySummary(entry))}</span><span class="entry-open"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 5 7 7-7 7"/></svg></span></button></div>${expanded ? `<section class="inline-entry" data-inline-entry="${escapeHTML(entry.insertId)}" data-render-settings="${escapeHTML(inlineSettings)}">${entryTools(entry, true)}${structuredEntry(entry)}<pre class="detail-code entry-raw" hidden>${escapeHTML(JSON.stringify(entry, null, 2))}</pre></section>` : ""}</div>`;
		})
		.join("");
	for (const node of list.querySelectorAll<HTMLElement>(
		"[data-inline-entry]",
	)) {
		const previous = previousInline.get(node.dataset.inlineEntry);
		if (previous) node.replaceWith(previous);
	}
	if (focused?.isConnected && list.contains(focused))
		focused.focus({ preventScroll: true });
}
