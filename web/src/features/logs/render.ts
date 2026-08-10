import { $, escapeHTML } from "../../shared/dom/selectors";
import { formatTime, severityClass } from "../../shared/format";
import { state } from "../../app/state";
import { t } from "../../shared/i18n/index";
import type { ExplorerEntry } from "../../shared/types";

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
	list.innerHTML = state.entries
		.map((entry) => {
			const containerName =
				entry.resource.labels.container_name || t("common.unknownContainer");
			return `<button class="entry-row ${state.selectedId === entry.insertId ? "selected" : ""}" type="button" data-entry-id="${escapeHTML(entry.insertId)}" aria-label="${escapeHTML(`${entry.severity} log from ${containerName}: ${entrySummary(entry)}`)}"><time class="entry-time" datetime="${escapeHTML(entry.timestamp)}">${escapeHTML(formatTime(entry.timestamp))}</time><span class="entry-severity ${severityClass(entry.severity)}">${escapeHTML(entry.severity)}</span><span class="entry-resource"><span class="resource-name">${escapeHTML(containerName)}</span><span class="resource-meta">${escapeHTML(entry.stream)} · ${escapeHTML(entry.resource.type)}</span></span><span class="entry-summary" title="${escapeHTML(entrySummary(entry))}">${escapeHTML(entrySummary(entry))}</span><span class="entry-open"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 5 7 7-7 7"/></svg></span></button>`;
		})
		.join("");
}
