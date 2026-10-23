import { getLocale } from "../../shared/i18n/index";
import { state } from "../../app/state";
import { syncURL } from "../../app/url-state";
import { $, escapeHTML as esc } from "../../shared/dom/selectors";
import { closeTail } from "../streaming/controller";
import { renderAll } from "../explorer/render";
import { renderFields } from "../fields/render";
import { renderDetail } from "../logs/detail";
import { w } from "./messages";
import {
	exportEntries,
	readQueries,
	shiftRange,
	snapshot,
	type QuerySnapshot,
	type StoredQuery,
} from "./model";

interface Hooks {
	reload: () => void;
	toast: (message: string) => void;
	closeDetail: () => void;
	changed: () => void;
}
let hooks: Hooks;
let libraryTab = "recent";
let workspaceLocale = "";
let libraryFilter = "";
let saved: StoredQuery[] = [];
let recent: StoredQuery[] = [];
const savedKey = "caroline-saved-queries";
const recentKey = "caroline-recent-queries";
const button = (id: string, label: string) =>
	`<button class="text-button" type="button" id="${id}">${esc(label)}</button>`;
const suggestions = (): StoredQuery[] =>
	[
		["errors", "severity >= WARNING"],
		["stderr", 'stream = "stderr"'],
		["structured", 'jsonPayload.message : "error"'],
		["all", ""],
	].map(([key, query]) => ({
		id: key,
		name: w(key as "errors" | "stderr" | "structured" | "all"),
		snapshot: {
			query,
			searchText: "",
			logName: "",
			resourceType: "",
			node: "",
			container: "",
			stream: "",
			severity: "",
			duration: "1h",
			timeFrom: "",
			timeTo: "",
		},
		updatedAt: "",
	}));

function persist(key: string, value: StoredQuery[]): boolean {
	try {
		window.localStorage.setItem(key, JSON.stringify(value));
		return true;
	} catch {
		hooks.toast(w("storageError"));
		return false;
	}
}
export function recordQuery(): void {
	if (!hooks) return;
	const value = snapshot(state);
	const name =
		[
			state.searchText,
			state.query,
			state.container,
			state.node,
			state.stream,
			state.severity,
		]
			.filter(Boolean)
			.join(" · ") || w("all");
	const next = [
		{
			id: crypto.randomUUID?.() || String(Date.now()),
			name,
			snapshot: value,
			updatedAt: new Date().toISOString(),
		},
		...recent.filter(
			(item) => JSON.stringify(item.snapshot) !== JSON.stringify(value),
		),
	].slice(0, 50);
	recent = next;
	persist(recentKey, next);
}
function commit(reload = true): void {
	state.pageToken = "";
	syncURL("push");
	renderAll();
	renderDetail();
	hooks.changed();
	if (reload) hooks.reload();
}
function absoluteRange(): [string, string] {
	return [
		state.timeFrom ||
			state.response?.from ||
			new Date(Date.now() - 300000).toISOString(),
		state.timeTo || state.response?.to || new Date().toISOString(),
	];
}
function setRange(from: string, to: string): void {
	state.timeFrom = from;
	state.timeTo = to;
	state.live = false;
	closeTail();
	commit();
}
function moveTime(direction: number): void {
	setRange(...shiftRange(...absoluteRange(), direction));
}
function extendTime(direction: number): void {
	const [from, to] = absoluteRange();
	const offset = Number(($("#extendDuration") as HTMLSelectElement).value);
	setRange(
		new Date(Date.parse(from) - (direction < 0 ? offset : 0)).toISOString(),
		new Date(Date.parse(to) + (direction > 0 ? offset : 0)).toISOString(),
	);
}
function applySnapshot(value: QuerySnapshot, run: boolean): void {
	hooks.closeDetail();
	const severity = [
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
	].includes(value.severity)
		? value.severity
		: "";
	Object.assign(state, value, {
		severity,
		draftQuery: value.query,
		showQuery: Boolean(value.query),
		live: false,
	});
	closeTail();
	state.entries = [];
	state.response = null;
	commit(run);
	if (!run) ($("#queryInput") as HTMLTextAreaElement).focus();
}
function createDialog(id: string): HTMLDialogElement {
	const dialog = document.createElement("dialog");
	dialog.id = id;
	dialog.className = "workspace-dialog";
	dialog.addEventListener("click", (event) => {
		if (event.target === dialog) {
			const bounds = dialog.getBoundingClientRect();
			if (
				event.clientX < bounds.left ||
				event.clientX > bounds.right ||
				event.clientY < bounds.top ||
				event.clientY > bounds.bottom
			)
				dialog.close();
		}
	});
	document.body.append(dialog);
	return dialog;
}
function dialogHeading(title: string): string {
	return `<header><h2 id="workspaceDialogTitle">${esc(title)}</h2><button type="button" class="icon-button" data-dialog-close aria-label="${esc(w("close"))}">×</button></header>`;
}
function openDialog(id: string, html: string): HTMLDialogElement {
	const dialog = $(id) as HTMLDialogElement;
	dialog.innerHTML = html
		.split("workspaceDialogTitle")
		.join(`${dialog.id}Title`);
	dialog.setAttribute("aria-labelledby", `${dialog.id}Title`);
	dialog
		.querySelector("[data-dialog-close]")
		?.addEventListener("click", () => dialog.close());
	if (!dialog.open) dialog.showModal();
	return dialog;
}
function showLibrary(): void {
	const dialog = openDialog(
		"#queryLibraryDialog",
		`${dialogHeading(w("library"))}<p>${esc(w("browserOnly"))}</p><nav class="library-tabs" aria-label="${esc(w("library"))}">${["recent", "saved", "suggested"].map((tab) => `<button type="button" class="text-button" data-library-tab="${tab}" aria-pressed="${libraryTab === tab}">${esc(w(tab as "recent"))}</button>`).join("")}</nav><label>${esc(w("query"))}<input type="search" id="librarySearch" placeholder="${esc(w("librarySearch"))}" value="${esc(libraryFilter)}" /></label><div id="queryLibraryList"></div>`,
	);
	dialog
		.querySelectorAll<HTMLButtonElement>("[data-library-tab]")
		.forEach((control) => {
			control.addEventListener("click", () => {
				libraryTab = control.dataset.libraryTab || "recent";
				dialog.querySelectorAll("[data-library-tab]").forEach((item) => {
					item.setAttribute(
						"aria-pressed",
						String((item as HTMLElement).dataset.libraryTab === libraryTab),
					);
				});
				renderLibrary();
			});
		});
	$("#librarySearch").addEventListener("input", (event) => {
		libraryFilter = (event.target as HTMLInputElement).value;
		renderLibrary();
	});
	renderLibrary();
}
function renderLibrary(): void {
	const items =
		libraryTab === "saved"
			? saved
			: libraryTab === "recent"
				? recent
				: suggestions();
	const filtered = items.filter((item) =>
		`${item.name} ${JSON.stringify(item.snapshot)}`
			.toLowerCase()
			.includes(libraryFilter.toLowerCase()),
	);
	$("#queryLibraryList").innerHTML = filtered.length
		? filtered
				.map(
					(item) =>
						`<article class="library-item"><strong>${esc(item.name)}</strong><code>${esc(item.snapshot.query || item.snapshot.searchText || w("all"))}</code><small>${esc(item.updatedAt ? new Date(item.updatedAt).toLocaleString() : "")}</small><div><button class="text-button" type="button" data-query-run="${esc(item.id)}">${esc(w("run"))}</button><button class="text-button" type="button" data-query-load="${esc(item.id)}">${esc(w("load"))}</button>${libraryTab !== "suggested" ? `<button class="text-button" type="button" data-query-delete="${esc(item.id)}">${esc(w("remove"))}</button>` : ""}</div></article>`,
				)
				.join("")
		: `<p>${esc(w("noQueries"))}</p>`;
	$("#queryLibraryList").onclick = (event) => {
		const control = (event.target as Element).closest<HTMLButtonElement>(
			"button",
		);
		if (!control) return;
		const id =
			control.dataset.queryRun ||
			control.dataset.queryLoad ||
			control.dataset.queryDelete;
		const item = items.find((item) => item.id === id);
		if (!item) return;
		if (control.dataset.queryDelete) {
			if (!window.confirm(w("deleteConfirm"))) return;
			const next = items.filter((item) => item.id !== id);
			if (persist(libraryTab === "saved" ? savedKey : recentKey, next)) {
				if (libraryTab === "saved") saved = next;
				else recent = next;
				renderLibrary();
			}
		} else {
			($("#queryLibraryDialog") as HTMLDialogElement).close();
			applySnapshot(item.snapshot, Boolean(control.dataset.queryRun));
		}
	};
}
function saveQuery(): void {
	const dialog = openDialog(
		"#saveQueryDialog",
		`${dialogHeading(w("save"))}<form id="saveQueryForm"><label>${esc(w("name"))}<input id="savedQueryName" name="queryName" maxlength="120" required autocomplete="off" /></label><p>${esc(w("browserOnly"))}</p><footer>${button("cancelSaveQuery", w("cancel"))}<button class="run-button" type="submit">${esc(w("saveSubmit"))}</button></footer></form>`,
	);
	$("#cancelSaveQuery").onclick = () => dialog.close();
	$("#saveQueryForm").onsubmit = (event) => {
		event.preventDefault();
		const input = $("#savedQueryName") as HTMLInputElement;
		const name = input.value.trim();
		if (!name) {
			input.setCustomValidity(w("nameRequired"));
			input.reportValidity();
			return;
		}
		const value = snapshot({ ...state, query: state.draftQuery });
		const next = [
			{
				id: crypto.randomUUID?.() || String(Date.now()),
				name,
				snapshot: value,
				updatedAt: new Date().toISOString(),
			},
			...saved,
		].slice(0, 100);
		if (persist(savedKey, next)) {
			saved = next;
			dialog.close();
			hooks.toast(w("savedNotice"));
		}
	};
	$("#savedQueryName").oninput = (event) =>
		(event.target as HTMLInputElement).setCustomValidity("");
}
function displaySettings(): void {
	const dialog = openDialog(
		"#displayDialog",
		`${dialogHeading(w("display"))}<form id="displayForm"><div class="workspace-checks">${[
			["displayFields", "fields", !state.fieldsHidden],
			["displayTimeline", "timeline", !state.timelineHidden],
			["displayCompact", "compact", state.compact],
			["displaySummary", "summaryVisible", state.showSummary],
			["displayMilliseconds", "milliseconds", state.showMilliseconds],
		]
			.map(
				([id, label, checked]) =>
					`<label><input type="checkbox" id="${id}" ${checked ? "checked" : ""} />${esc(w(label as "fields"))}</label>`,
			)
			.join(
				"",
			)}</div><label>${esc(w("summaryFields"))}<textarea id="summaryFieldsInput" rows="4" spellcheck="false">${esc(state.summaryFields.join("\n"))}</textarea></label><p>${esc(w("summaryHelp"))}</p><label>${esc(w("fieldWidth"))}<input type="range" id="fieldsWidth" min="180" max="440" step="10" value="${state.fieldsWidth}" /></label><label>${esc(w("chartHeight"))}<input type="range" id="chartHeight" min="80" max="360" step="20" value="${state.timelineHeight}" /></label><footer>${button("resetDisplay", w("reset"))}<button class="run-button" type="submit">${esc(w("apply"))}</button></footer></form>`,
	);
	$("#resetDisplay").onclick = () => {
		state.summaryFields = [];
		state.compact = false;
		state.showSummary = true;
		state.showMilliseconds = true;
		state.fieldsHidden = false;
		state.timelineHidden = false;
		state.fieldsWidth = 235;
		state.timelineHeight = 120;
		dialog.close();
		commit(false);
	};
	$("#displayForm").onsubmit = (event) => {
		event.preventDefault();
		const input = $("#summaryFieldsInput") as HTMLTextAreaElement;
		const fields = input.value
			.split("\n")
			.map((item) => item.trim())
			.filter(Boolean);
		if (fields.length > 8 || fields.some((field) => !/^[\w.-]+$/.test(field))) {
			input.setCustomValidity(w("summaryInvalid"));
			input.reportValidity();
			return;
		}
		state.summaryFields = [...new Set(fields)];
		state.fieldsHidden = !($("#displayFields") as HTMLInputElement).checked;
		state.timelineHidden = !($("#displayTimeline") as HTMLInputElement).checked;
		state.compact = ($("#displayCompact") as HTMLInputElement).checked;
		state.showSummary = ($("#displaySummary") as HTMLInputElement).checked;
		state.showMilliseconds = (
			$("#displayMilliseconds") as HTMLInputElement
		).checked;
		state.fieldsWidth = Number(($("#fieldsWidth") as HTMLInputElement).value);
		state.timelineHeight = Number(
			($("#chartHeight") as HTMLInputElement).value,
		);
		dialog.close();
		commit(false);
	};
	$("#summaryFieldsInput").oninput = (event) =>
		(event.target as HTMLTextAreaElement).setCustomValidity("");
}
function download(): void {
	if (!state.entries.length) {
		hooks.toast(w("noEntries"));
		return;
	}
	const dialog = openDialog(
		"#downloadDialog",
		`${dialogHeading(w("download"))}<p>${esc(w("exportHelp"))}</p><p>${esc(w("count"))}: ${state.entries.length.toLocaleString()}</p><label>${esc(w("format"))}<select id="downloadFormat"><option value="json">JSON</option><option value="csv">CSV</option></select></label><footer>${button("downloadResults", w("downloadAction"))}</footer>`,
	);
	$("#downloadResults").onclick = () => {
		const format = ($("#downloadFormat") as HTMLSelectElement).value;
		const url = URL.createObjectURL(
			new Blob([exportEntries(state.entries, format)], {
				type:
					format === "json"
						? "application/json;charset=utf-8"
						: "text/csv;charset=utf-8",
			}),
		);
		const link = document.createElement("a");
		link.href = url;
		link.download = `caroline-${new Date().toISOString().replace(/[:.]/g, "-")}.${format}`;
		link.click();
		window.setTimeout(() => URL.revokeObjectURL(url), 1000);
		dialog.close();
	};
}
export function setupWorkspace(nextHooks: Hooks): void {
	hooks = nextHooks;
	try {
		saved = readQueries(window.localStorage, savedKey);
		recent = readQueries(window.localStorage, recentKey);
	} catch {
		/* Browser storage is optional. */
	}
	const heading = $("#explorerView .heading-actions");
	heading.insertAdjacentHTML(
		"afterbegin",
		`${button("queryLibraryButton", w("library"))}${button("saveQueryButton", w("save"))}${button("displaySettingsButton", w("display"))}`,
	);
	$("#severityFilter").insertAdjacentHTML(
		"beforeend",
		["DEFAULT", "NOTICE", "CRITICAL", "ALERT", "EMERGENCY"]
			.map((value) => `<option value="${value}">${value}</option>`)
			.join(""),
	);
	const severitySelect = $("#severityFilter") as HTMLSelectElement;
	[...severitySelect.options]
		.sort(
			(a, b) =>
				[
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
				].indexOf(a.value) -
				[
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
				].indexOf(b.value),
		)
		.forEach((option) => {
			severitySelect.append(option);
		});
	$(".refine-row").insertAdjacentHTML(
		"beforeend",
		`<label class="filter-control"><span>${esc(w("resourceType"))}</span><select id="resourceTypeFilter"></select></label><label class="filter-control"><span>${esc(w("logName"))}</span><select id="logNameFilter"></select></label>`,
	);
	for (const [id, key] of [
		["logNameFilter", "logName"],
		["resourceTypeFilter", "resourceType"],
	] as const)
		$(`#${id}`).onchange = (event) => {
			state[key] = (event.target as HTMLSelectElement).value;
			commit();
		};
	$(".query-search-row").insertAdjacentHTML(
		"afterend",
		`<div class="explorer-tools time-navigation">${button("previousTimeButton", `← ${w("previous")}`)}${button("nowTimeButton", w("now"))}${button("nextTimeButton", `${w("next")} →`)}<label>${esc(w("zone"))}<select id="timeZoneSelect"><option value="local">${esc(w("local"))}</option><option value="UTC">UTC</option></select></label></div>`,
	);
	$(".results-header").insertAdjacentHTML(
		"afterend",
		`<div class="explorer-tools results-tools">${button("expandAllButton", w("expand"))}${button("collapseAllButton", w("collapse"))}${button("downloadButton", w("download"))}<label class="result-highlight"><span class="sr-only">${esc(w("highlight"))}</span><input type="search" id="resultHighlight" placeholder="${esc(w("highlight"))}" /></label></div>`,
	);
	$(".results-footer").insertAdjacentHTML(
		"beforebegin",
		`<div class="explorer-tools extend-tools">${button("extendOlderButton", w("older"))}<label><span class="sr-only">${esc(w("extend"))}</span><select id="extendDuration"><option value="60000">${esc(w("minute"))}</option><option value="300000">${esc(w("fiveMinutes"))}</option><option value="3600000">${esc(w("hour"))}</option></select></label>${button("extendNewerButton", w("newer"))}<output id="loadedResultCount" aria-live="polite"></output></div>`,
	);
	$("#fields .panel-heading").insertAdjacentHTML(
		"afterend",
		`<label class="field-search"><span class="sr-only">${esc(w("fieldSearch"))}</span><input type="search" id="fieldSearch" placeholder="${esc(w("fieldSearch"))}" /></label>`,
	);
	for (const id of [
		"queryLibraryDialog",
		"saveQueryDialog",
		"displayDialog",
		"downloadDialog",
	])
		createDialog(id);
	$("#queryLibraryButton").onclick = showLibrary;
	$("#saveQueryButton").onclick = saveQuery;
	$("#displaySettingsButton").onclick = displaySettings;
	$("#previousTimeButton").onclick = () => moveTime(-1);
	$("#nextTimeButton").onclick = () => moveTime(1);
	$("#nowTimeButton").onclick = () => {
		const [from, to] = absoluteRange();
		const end = new Date();
		setRange(
			new Date(
				end.getTime() - (Date.parse(to) - Date.parse(from)),
			).toISOString(),
			end.toISOString(),
		);
	};
	$("#extendOlderButton").onclick = () => extendTime(-1);
	$("#extendNewerButton").onclick = () => extendTime(1);
	$("#downloadButton").onclick = download;
	$("#fieldSearch").oninput = renderFields;
	$("#expandAllButton").onclick = () => {
		state.expandedEntries = ["*"];
		commit(false);
	};
	$("#collapseAllButton").onclick = () => {
		state.expandedEntries = [];
		commit(false);
	};
	$("#resultHighlight").oninput = (event) => {
		state.highlight = (event.target as HTMLInputElement).value;
		syncURL();
		renderAll();
	};
	const supportedZones = (
		Intl as unknown as { supportedValuesOf?: (key: string) => string[] }
	).supportedValuesOf?.("timeZone") || [
		"Asia/Tokyo",
		"America/New_York",
		"Europe/London",
	];
	for (const zone of supportedZones)
		($("#timeZoneSelect") as HTMLSelectElement).add(new Option(zone, zone));
	$("#timeZoneSelect").onchange = (event) => {
		state.timeZone = (event.target as HTMLSelectElement).value;
		commit(false);
	};
	window.addEventListener("storage", (event) => {
		if (event.key === savedKey) {
			saved = readQueries(localStorage, savedKey);
			if (($("#queryLibraryDialog") as HTMLDialogElement).open) renderLibrary();
		}
	});
}
export function renderWorkspace(): void {
	document.documentElement.style.setProperty(
		"--fields-width",
		`${state.fieldsWidth}px`,
	);
	document.documentElement.style.setProperty(
		"--chart-height",
		`${state.timelineHeight}px`,
	);
	const zone = document.querySelector<HTMLSelectElement>("#timeZoneSelect");
	if (!zone) return;
	if (![...zone.options].some((option) => option.value === state.timeZone))
		zone.add(new Option(state.timeZone, state.timeZone));
	zone.value = state.timeZone;
	if (workspaceLocale !== getLocale()) {
		workspaceLocale = getLocale();
		for (const [id, key] of [
			["queryLibraryButton", "library"],
			["saveQueryButton", "save"],
			["displaySettingsButton", "display"],
			["previousTimeButton", "previous"],
			["nextTimeButton", "next"],
			["nowTimeButton", "now"],
			["expandAllButton", "expand"],
			["collapseAllButton", "collapse"],
			["downloadButton", "download"],
			["extendOlderButton", "older"],
			["extendNewerButton", "newer"],
			["cancelQueryButton", "cancelQuery"],
		] as const) {
			const control = document.getElementById(id);
			if (control) control.textContent = w(key);
		}
		for (const [id, key] of [
			["resultHighlight", "highlight"],
			["fieldSearch", "fieldSearch"],
		] as const) {
			const input = $(`#${id}`) as HTMLInputElement;
			input.placeholder = w(key);
			input.setAttribute("aria-label", w(key));
		}
		zone.options[0].textContent = w("local");
		const label = zone.closest("label")?.firstChild;
		if (label) label.textContent = w("zone");
		for (const [id, key] of [
			["logNameFilter", "logName"],
			["resourceTypeFilter", "resourceType"],
		] as const) {
			const label = $(`#${id}`).closest("label")?.querySelector("span");
			if (label) label.textContent = w(key);
		}
	}

	for (const [id, key, field, title] of [
		["logNameFilter", "logName", "logName", "allLogNames"],
		["resourceTypeFilter", "resourceType", "resource.type", "allResources"],
	] as const) {
		const select = $(`#${id}`) as HTMLSelectElement;
		const values = new Set<string>(
			[...select.options].map((option) => option.value).filter(Boolean),
		);
		for (const group of state.response?.fields || [])
			for (const item of group.fields)
				if (item.name === field)
					for (const value of Object.keys(item.values || {})) values.add(value);
		if (state[key]) values.add(state[key]);
		select.innerHTML =
			`<option value="">${esc(w(title))}</option>` +
			[...values]
				.sort()
				.map((value) => `<option value="${esc(value)}">${esc(value)}</option>`)
				.join("");
		select.value = state[key];
	}
	const highlight = $("#resultHighlight") as HTMLInputElement;
	if (document.activeElement !== highlight) highlight.value = state.highlight;
	$("#loadedResultCount").textContent =
		`${w("count")}: ${state.entries.length.toLocaleString()}`;
	document.body.classList.toggle("compact-logs", state.compact);
	document.body.classList.toggle("hide-log-summary", !state.showSummary);
}
