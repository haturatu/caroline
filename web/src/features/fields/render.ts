import { $, escapeHTML as esc } from "../../shared/dom/selectors";
import { formatNumber } from "../../shared/format";
import { state } from "../../app/state";
import { t } from "../../shared/i18n/index";
import { w } from "../workspace/messages";
import { getRenderActions } from "../explorer/actions";

const allValues = new Set<string>();
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
	const search =
		document
			.querySelector<HTMLInputElement>("#fieldSearch")
			?.value.trim()
			.toLowerCase() || "";
	target.innerHTML =
		state.response.fields
			.map((group) => {
				const fields = group.fields
					.map((field) => {
						const matchesName = field.name.toLowerCase().includes(search);
						const values = Object.entries(field.values || {})
							.filter(
								([value]) =>
									matchesName || value.toLowerCase().includes(search),
							)
							.sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
						if (search && !matchesName && !values.length) return "";
						const expanded =
							Boolean(search) || state.expandedFields.includes(field.name);
						const visible =
							allValues.has(field.name) || search ? values : values.slice(0, 5);
						return `<section><button class="field-row${expanded ? " expanded" : ""}" type="button" aria-expanded="${expanded}" data-field="${esc(field.name)}"><span class="field-name" title="${esc(field.name)}">${esc(field.name)}</span><span class="field-count">${formatNumber(field.count)}</span></button><div class="field-values">${visible.map(([value, count]) => `<div class="field-value-row"><button class="field-value" type="button" data-filter-field="${esc(field.name)}" data-value="${esc(value)}" title="${esc(w("include"))}">${esc(value)} <span>(${formatNumber(count)})</span></button><button class="field-exclude icon-button" type="button" data-filter-field="${esc(field.name)}" data-value="${esc(value)}" data-exclude="true" aria-label="${esc(`${w("exclude")}: ${value}`)}">−</button></div>`).join("")}${values.length > 5 && !search ? `<button class="text-button" type="button" data-more-values="${esc(field.name)}">${esc(w(allValues.has(field.name) ? "showLess" : "showMore"))} (${values.length})</button>` : ""}</div></section>`;
					})
					.join("");
				const name =
					group.name === "System Metadata"
						? t("fields.systemMetadata")
						: group.name === "Frequent Fields"
							? t("fields.frequentFields")
							: group.name;
				return fields
					? `<section class="field-group"><div class="field-group-title">${esc(name)}</div>${fields}</section>`
					: "";
			})
			.join("") || `<p class="panel-loading">${esc(w("noFields"))}</p>`;
	target.onclick = (event) => {
		const control = (event.target as Element).closest<HTMLButtonElement>(
			"button",
		);
		if (!control) return;
		const field = control.dataset.field;
		if (field) {
			const expanded = !state.expandedFields.includes(field);
			state.expandedFields = expanded
				? [...state.expandedFields, field]
				: state.expandedFields.filter((item) => item !== field);
			control.classList.toggle("expanded", expanded);
			control.setAttribute("aria-expanded", String(expanded));
			return;
		}
		if (control.dataset.moreValues) {
			const name = control.dataset.moreValues;
			if (allValues.has(name)) allValues.delete(name);
			else allValues.add(name);
			renderFields();
			target
				.querySelector<HTMLButtonElement>(
					`[data-more-values="${CSS.escape(name)}"]`,
				)
				?.focus();
			return;
		}
		if (control.dataset.filterField)
			getRenderActions().onFieldFilter?.(
				control.dataset.filterField,
				control.dataset.value || "",
				control.dataset.exclude === "true",
			);
	};
}
