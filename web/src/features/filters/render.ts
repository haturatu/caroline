import { buildBasicQuery } from "../explorer/query";
import { $, escapeHTML } from "../../shared/dom/selectors";
import { state } from "../../app/state";
import { t } from "../../shared/i18n/index";

export function renderFilters(): void {
	const select = $("#containerFilter") as HTMLSelectElement;
	const options = [
		`<option value="">${t("filters.allContainers")}</option>`,
		...state.containers.map(
			(container) =>
				`<option value="${escapeHTML(container.id)}">${escapeHTML(container.name)}</option>`,
		),
	];
	select.innerHTML = options.join("");
	select.value = state.container;
	($("#streamFilter") as HTMLSelectElement).value = state.stream;
	($("#severityFilter") as HTMLSelectElement).value = state.severity;
	($("#rangeFilter") as HTMLSelectElement).value = state.timeFrom
		? "custom"
		: state.duration;
	const customRange = $("#customRangeEditor");
	customRange.toggleAttribute("hidden", !state.timeFrom || !state.timeTo);
	($("#customFromInput") as HTMLInputElement).value = state.timeFrom
		? toDateTimeLocal(state.timeFrom)
		: "";
	($("#customToInput") as HTMLInputElement).value = state.timeTo
		? toDateTimeLocal(state.timeTo)
		: "";
	($("#sortFilter") as HTMLSelectElement).value = state.sort;
	($("#queryInput") as HTMLTextAreaElement).value = state.draftQuery;
	($("#searchAllFields") as HTMLInputElement).value = state.searchText;
	$("#combinedQueryPreview").textContent =
		[...buildBasicQuery(), state.draftQuery.trim()]
			.filter(Boolean)
			.join(" AND ") || t("query.allLogs");
	$("#queryEditor").toggleAttribute("hidden", !state.showQuery);
	$("#showQueryButton").textContent = state.showQuery
		? t("query.hideQuery")
		: t("query.showQuery");
}

function toDateTimeLocal(value: string): string {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return "";
	const pad = (part: number) => String(part).padStart(2, "0");
	return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
