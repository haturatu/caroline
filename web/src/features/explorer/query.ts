import { state } from "../../app/state";

function quoteQueryValue(value: string): string {
	return `"${value.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}

/** Basic controls and the advanced editor are sent as one visible query. */
export function buildBasicQuery(): string[] {
	const parts: string[] = [];
	if (state.container) {
		const container = state.containers.find(
			(item) => item.id === state.container,
		);
		if (container) {
			parts.push(
				`resource.labels.container_name = ${quoteQueryValue(container.name)}`,
			);
		}
	}
	if (state.stream) parts.push(`stream = ${quoteQueryValue(state.stream)}`);
	if (state.severity) parts.push(`severity = ${state.severity}`);
	if (state.searchText)
		parts.push(`SEARCH(${quoteQueryValue(state.searchText.trim())})`);
	return parts;
}

export function buildExplorerQuery(): string {
	return [...buildBasicQuery(), state.query.trim()]
		.filter(Boolean)
		.join(" AND ");
}
