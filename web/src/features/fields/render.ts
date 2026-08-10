import { $$, $, escapeHTML } from "../../shared/dom/selectors";
import { formatNumber } from "../../shared/format";
import { state } from "../../app/state";
import { t } from "../../shared/i18n/index";
import { getRenderActions } from "../explorer/actions";

export function renderFields(): void {
	const target = $("#fieldGroups");
	if (state.loading && !state.response) {
		target.innerHTML = `<div class="panel-loading loading-panel"><span class="spinner"></span><span>${t("fields.loading")}</span></div>`;
		return;
	}
	if (!state.response?.fields.length) {
		target.innerHTML = `<div class="panel-loading">${t("fields.noFields")}</div>`;
		return;
	}
	const availableFields = new Set(
		state.response.fields.flatMap((group) =>
			group.fields.map((field) => field.name),
		),
	);
	state.expandedFields = state.expandedFields.filter((field) =>
		availableFields.has(field),
	);
	const expandedFields = new Set(state.expandedFields);
	target.innerHTML = state.response.fields
		.map((group) => {
			const fields = group.fields
				.map((field) => {
					const isExpanded = expandedFields.has(field.name);
					const values = Object.entries(field.values || {})
						.slice(0, 5)
						.map(
							([value, count]) =>
								`<button class="field-value" type="button" data-field="${escapeHTML(field.name)}" data-value="${escapeHTML(value)}">${escapeHTML(value)} <span>(${formatNumber(count)})</span></button>`,
						)
						.join("");
					return `<button class="field-row${isExpanded ? " expanded" : ""}" type="button" aria-expanded="${isExpanded}" data-field="${escapeHTML(field.name)}"><span class="field-name" title="${escapeHTML(field.name)}">${escapeHTML(field.name)}</span><span class="field-count">${formatNumber(field.count)}</span></button><div class="field-values">${values}</div>`;
				})
				.join("");
			const groupName =
				group.name === "System Metadata"
					? t("fields.systemMetadata")
					: group.name === "Frequent Fields"
						? t("fields.frequentFields")
						: group.name;
			return `<section class="field-group"><div class="field-group-title">${escapeHTML(groupName)}</div>${fields}</section>`;
		})
		.join("");
	$$<HTMLButtonElement>(".field-row").forEach((button) => {
		button.addEventListener("click", () => {
			const expanded = !button.classList.contains("expanded");
			const field = button.dataset.field;
			button.classList.toggle("expanded", expanded);
			button.setAttribute("aria-expanded", String(expanded));
			if (field) {
				const fields = new Set(state.expandedFields);
				if (expanded) fields.add(field);
				else fields.delete(field);
				state.expandedFields = [...fields];
			}
		});
	});
	$$<HTMLButtonElement>(".field-value").forEach((button) => {
		button.addEventListener("click", () => {
			const field = button.getAttribute("data-field");
			const value = button.getAttribute("data-value");
			if (field && value) getRenderActions().onFieldFilter?.(field, value);
		});
	});
}
