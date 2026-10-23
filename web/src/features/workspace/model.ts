import type { ExplorerEntry } from "../../shared/types";

export interface QuerySnapshot {
	query: string;
	searchText: string;
	logName: string;
	resourceType: string;
	node: string;
	container: string;
	stream: string;
	severity: string;
	duration: string;
	timeFrom: string;
	timeTo: string;
}
export interface StoredQuery {
	id: string;
	name: string;
	snapshot: QuerySnapshot;
	updatedAt: string;
}
export const queryKeys = [
	"query",
	"searchText",
	"logName",
	"resourceType",
	"node",
	"container",
	"stream",
	"severity",
	"duration",
	"timeFrom",
	"timeTo",
] as const;
export function snapshot(source: QuerySnapshot): QuerySnapshot {
	return Object.fromEntries(
		queryKeys.map((key) => [key, source[key]]),
	) as unknown as QuerySnapshot;
}
export function readQueries(
	storage: Pick<Storage, "getItem">,
	key: string,
): StoredQuery[] {
	try {
		const data: unknown = JSON.parse(storage.getItem(key) || "[]");
		if (!Array.isArray(data)) return [];
		return data
			.filter((item): item is StoredQuery =>
				Boolean(
					item &&
						typeof item.id === "string" &&
						typeof item.name === "string" &&
						typeof item.updatedAt === "string" &&
						item.snapshot &&
						queryKeys.every((key) => typeof item.snapshot[key] === "string"),
				),
			)
			.slice(0, 100);
	} catch {
		return [];
	}
}
export function valueAt(entry: ExplorerEntry, path: string): unknown {
	let current: unknown = entry;
	for (const key of path.split(".")) {
		if (
			!current ||
			typeof current !== "object" ||
			!Object.getOwnPropertyDescriptor(current, key)
		)
			return undefined;
		current = (current as Record<string, unknown>)[key];
	}
	return current;
}
export function displayValue(value: unknown): string {
	return typeof value === "string" ? value : (JSON.stringify(value) ?? "");
}
export function csvCell(value: unknown): string {
	const text = displayValue(value);
	// Spreadsheet programs interpret these prefixes as formulas, even in quoted CSV cells.
	return `"${(/^[\s]*[=+\-@\t\r]/.test(text) ? "'" : "") + text.replace(/"/g, '""')}"`;
}
export function exportEntries(
	entries: ExplorerEntry[],
	format: string,
): string {
	if (format === "json") return JSON.stringify(entries, null, 2);
	const columns = [
		"timestamp",
		"severity",
		"resource.labels.node_name",
		"resource.labels.container_name",
		"stream",
		"logName",
		"textPayload",
		"jsonPayload",
		"insertId",
	];
	return [
		columns.map(csvCell).join(","),
		...entries.map((entry) =>
			columns.map((path) => csvCell(valueAt(entry, path) ?? "")).join(","),
		),
	].join("\r\n");
}
export function shiftRange(
	from: string,
	to: string,
	direction: number,
): [string, string] {
	const start = Date.parse(from),
		end = Date.parse(to);
	const offset = (end - start) * direction;
	return [
		new Date(start + offset).toISOString(),
		new Date(end + offset).toISOString(),
	];
}
