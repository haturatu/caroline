import { state } from "../../app/state";
import { buildExplorerQuery } from "../explorer/query";
import type { ExplorerEntry } from "../../shared/types";

export function buildTailURL(since: string): string {
	const params = new URLSearchParams({ since });
	const query = buildExplorerQuery();
	if (query) params.set("q", query);
	if (state.container) params.set("containers", state.container);
	if (state.node) params.set("nodes", state.node);
	if (state.stream) params.set("stream", state.stream);
	if (state.severity) params.set("severity", state.severity);
	return `/api/tail?${params.toString()}`;
}

export type TailHandlers = {
	onOpen: () => void;
	onEntry: (entry: ExplorerEntry) => void;
	onWarning: (message: string) => void;
	onServerError: (message: string) => void;
	onDisconnect: () => void;
};

function eventPayload<T>(event: Event): T | null {
	const data = (event as MessageEvent<string>).data;
	if (!data) return null;
	try {
		return JSON.parse(data) as T;
	} catch {
		return null;
	}
}

export function openTail(since: string, handlers: TailHandlers): EventSource {
	const source = new EventSource(buildTailURL(since));
	source.addEventListener("open", handlers.onOpen);
	source.addEventListener("log", (event) => {
		const entry = eventPayload<ExplorerEntry>(event);
		if (entry) handlers.onEntry(entry);
	});
	source.addEventListener("warning", (event) => {
		const payload = eventPayload<{ message?: string }>(event);
		if (payload?.message) handlers.onWarning(payload.message);
	});
	source.addEventListener("error", (event) => {
		const payload = eventPayload<{ message?: string }>(event);
		if (payload?.message) handlers.onServerError(payload.message);
		else handlers.onDisconnect();
	});
	return source;
}
