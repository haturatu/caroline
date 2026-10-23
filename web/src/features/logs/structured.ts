import { t } from "../../shared/i18n/index";
import { state } from "../../app/state";
import { syncURL } from "../../app/url-state";
import { copyText } from "../../shared/dom/clipboard";
import { escapeHTML as esc } from "../../shared/dom/selectors";
import type { ExplorerEntry } from "../../shared/types";
import { getRenderActions } from "../explorer/actions";
import { w } from "../workspace/messages";
import { displayValue } from "../workspace/model";

export function highlighted(text: string): string {
	const needle = state.highlight.trim();
	if (!needle) return esc(text);
	let offset = 0;
	let result = "";
	const lower = text.toLowerCase(),
		term = needle.toLowerCase();
	let index = lower.indexOf(term);
	while (index >= 0) {
		result += `${esc(text.slice(offset, index))}<mark>${esc(text.slice(index, index + term.length))}</mark>`;
		offset = index + term.length;
		index = lower.indexOf(term, offset);
	}
	return result + esc(text.slice(offset));
}
export function structuredEntry(entry: ExplorerEntry): string {
	const visit = (value: unknown, path: string, depth: number): string => {
		if (value !== null && typeof value === "object" && depth < 20) {
			const values = Object.entries(value);
			return `<details class="log-tree-node" ${depth < 1 ? "open" : ""}><summary>${esc(path.split(".").pop() || "log")} <span class="muted">{${values.length}}</span></summary><div>${values.map(([key, item]) => visit(item, path ? `${path}.${key}` : key, depth + 1)).join("")}</div></details>`;
		}
		const display = displayValue(value);
		const filterable =
			/^(severity|timestamp|stream|logName|resource\.type|resource\.labels\.[\w.-]+|labels\.[\w.-]+|jsonPayload\.[\w.-]+|textPayload)$/.test(
				path,
			) && value !== null;
		return `<div class="log-tree-value"><code title="${esc(path)}">${esc(path.split(".").pop() || path)}</code><span class="log-tree-text">${highlighted(display)}</span><details class="field-actions"><summary aria-label="${esc(path)}">⋮</summary><div>${filterable ? `<button type="button" data-log-action="include" data-path="${esc(path)}" data-value="${esc(display)}">${esc(w("include"))}</button><button type="button" data-log-action="exclude" data-path="${esc(path)}" data-value="${esc(display)}">${esc(w("exclude"))}</button>` : ""}<button type="button" data-log-action="copy" data-value="${esc(display)}">${esc(w("copyValue"))}</button><button type="button" data-log-action="summary" data-path="${esc(path)}">${esc(w(state.summaryFields.includes(path) ? "removeSummary" : "addSummary"))}</button></div></details></div>`;
	};
	return `<div class="log-tree" translate="no">${visit(entry, "", 0)}</div>`;
}
export function entryTools(entry: ExplorerEntry, inline = false): string {
	const index = state.entries.findIndex(
		(item) => item.insertId === entry.insertId,
	);
	return `<div class="entry-tools" data-tools-entry="${esc(entry.insertId)}">${inline ? `<button class="text-button" type="button" data-log-action="details">${esc(w("details"))}</button>` : `<button class="text-button" type="button" data-log-action="previous" ${index <= 0 ? "disabled" : ""}>← ${esc(w("previousEntry"))}</button><button class="text-button" type="button" data-log-action="next" ${index >= state.entries.length - 1 ? "disabled" : ""}>${esc(w("nextEntry"))} →</button>`}<button class="text-button" type="button" data-log-action="link">${esc(w("copyLink"))}</button><button class="text-button" type="button" data-log-action="context">${esc(w("context"))}</button><button class="text-button" type="button" data-log-action="expand">${esc(w("expandNested"))}</button><button class="text-button" type="button" data-log-action="collapse">${esc(w("collapseNested"))}</button><button class="text-button" type="button" data-log-action="raw">${esc(w("raw"))}</button></div>`;
}
export function handleLogAction(
	event: MouseEvent,
	openDetail: (id: string) => void,
	rerender: () => void,
): boolean {
	const control = (event.target as Element).closest<HTMLButtonElement>(
		"[data-log-action]",
	);
	if (!control) return false;
	const action = control.dataset.logAction;
	const path = control.dataset.path || "";
	const value = control.dataset.value || "";
	const entryId =
		control.closest<HTMLElement>("[data-inline-entry]")?.dataset.inlineEntry ||
		state.selectedId;
	const entry = state.entries.find((item) => item.insertId === entryId);
	const actions = getRenderActions();
	if (action === "include" || action === "exclude") {
		actions.onFieldFilter?.(path, value, action === "exclude");
		return true;
	}
	if (action === "copy") {
		void copyText(value).then((copied) =>
			actions.onToast?.(copied ? t("detail.copied") : t("detail.copyFailed")),
		);
		return true;
	}
	if (action === "summary") {
		state.summaryFields = state.summaryFields.includes(path)
			? state.summaryFields.filter((item) => item !== path)
			: [...state.summaryFields, path].slice(-8);
		syncURL();
		rerender();
		return true;
	}
	if (!entry) return true;
	if (action === "details") openDetail(entry.insertId);
	if (action === "previous" || action === "next") {
		const index =
			state.entries.indexOf(entry) + (action === "previous" ? -1 : 1);
		if (state.entries[index]) openDetail(state.entries[index].insertId);
	}
	if (action === "context") actions.onContext?.(entry);
	if (action === "link") {
		const url = new URL(window.location.href);
		url.searchParams.set("entry", entry.insertId);
		url.searchParams.set("from", entry.timestamp);
		url.searchParams.set(
			"to",
			new Date(Date.parse(entry.timestamp) + 60000).toISOString(),
		);
		url.searchParams.set("sort", "asc");
		url.searchParams.set("live", "0");
		url.searchParams.delete("pageToken");
		void copyText(url.href).then((copied) =>
			actions.onToast?.(
				copied ? t("common.linkCopied") : t("detail.copyFailed"),
			),
		);
	}
	const scope =
		control.closest<HTMLElement>("[data-inline-entry]") ||
		document.querySelector("#detailBody");
	if (action === "expand" || action === "collapse")
		scope
			?.querySelectorAll<HTMLDetailsElement>(".log-tree-node")
			.forEach((node) => {
				node.open = action === "expand";
			});
	if (action === "raw") {
		const tree = scope?.querySelector<HTMLElement>(".log-tree");
		const raw = scope?.querySelector<HTMLElement>(".entry-raw");
		if (tree && raw) {
			const showRaw = raw.hidden;
			tree.hidden = showRaw;
			raw.hidden = !showRaw;
			control.textContent = w(showRaw ? "tree" : "raw");
		}
	}
	return true;
}
