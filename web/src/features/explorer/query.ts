import { state } from "../../app/state";

function quoteQueryValue(value: string): string {
	return JSON.stringify(value);
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
	if (state.logName) parts.push(`logName = ${quoteQueryValue(state.logName)}`);
	if (state.resourceType)
		parts.push(`resource.type = ${quoteQueryValue(state.resourceType)}`);
	if (state.stream) parts.push(`stream = ${quoteQueryValue(state.stream)}`);
	if (state.severity) parts.push(`severity = ${state.severity}`);
	if (state.searchText)
		parts.push(`SEARCH(${quoteQueryValue(state.searchText.trim())})`);
	return parts;
}

export function buildExplorerQuery(): string {
	const basic = buildBasicQuery().join(" AND ");
	const advanced = state.query.trim();
	return basic && advanced ? `${basic} AND (${advanced})` : basic || advanced;
}
