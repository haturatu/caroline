import { state } from "../../app/state";
import { renderDetail } from "../logs/detail";
import { renderEntries } from "../logs/render";
import { renderResultsMeta } from "../explorer/render";
import { openTail } from "./sse";
import { t } from "../../shared/i18n/index";
import type { ExplorerEntry } from "../../shared/types";

let tailSource: EventSource | null = null;

export function closeTail(): void {
	if (tailSource) tailSource.close();
	tailSource = null;
	state.tailConnected = false;
	state.tailMessage = "";
}

function appendTailEntry(entry: ExplorerEntry): void {
	if (state.entries.some((item) => item.insertId === entry.insertId)) return;
	const entries = [...state.entries, entry].sort((left, right) => {
		const timestampOrder =
			Date.parse(left.timestamp) - Date.parse(right.timestamp);
		if (timestampOrder !== 0)
			return state.sort === "asc" ? timestampOrder : -timestampOrder;
		if (state.sort === "asc")
			return left.insertId.localeCompare(right.insertId);
		return right.insertId.localeCompare(left.insertId);
	});
	const entryLimit = state.response?.entryLimit || 50000;
	state.entries =
		entries.length <= entryLimit
			? entries
			: state.sort === "asc"
				? entries.slice(entries.length - entryLimit)
				: entries.slice(0, entryLimit);
	if (state.response) {
		state.response = {
			...state.response,
			total: state.response.total + 1,
		};
	}
	state.lastUpdated = new Date().toISOString();
	renderEntries();
	renderDetail();
	renderResultsMeta();
}

export function startTail(
	since: string,
	onError: (message: string) => void,
): void {
	if (!state.live) return;
	closeTail();
	state.tailMessage = t("results.connecting");
	tailSource = openTail(since, {
		onOpen: () => {
			state.tailConnected = true;
			state.tailMessage = t("results.liveConnected");
			renderResultsMeta();
		},
		onEntry: appendTailEntry,
		onWarning: (message) => {
			state.tailMessage = message;
			renderResultsMeta();
		},
		onServerError: (message) => {
			state.tailMessage = t("errors.liveError");
			onError(message);
			renderResultsMeta();
		},
		onDisconnect: () => {
			state.tailConnected = false;
			state.tailMessage = t("errors.reconnecting");
			renderResultsMeta();
		},
	});
	renderResultsMeta();
}
