import { en, type Messages } from "./locales/en";
import { ja } from "./locales/ja";
import { ru } from "./locales/ru";
import { zhCN } from "./locales/zh-CN";
import { zhTW } from "./locales/zh-TW";
import {
	detectLocale,
	saveLocale,
	type Locale,
	supportedLocales,
} from "./locale";

const locales: Record<Locale, Messages> = {
	en,
	ja,
	"zh-CN": zhCN,
	"zh-TW": zhTW,
	ru,
};

interface MessageTree {
	[key: string]: string | MessageTree;
}

type MessageValue = string | MessageTree;

let currentLocale: Locale = detectLocale();

function resolve(messages: Messages, key: string): MessageValue | undefined {
	let value: MessageValue = messages;
	for (const part of key.split(".")) {
		if (typeof value !== "object" || !(part in value)) return undefined;
		value = value[part];
	}
	return value === messages ? undefined : value;
}

function interpolate(
	value: string,
	values: Record<string, string | number>,
): string {
	return value.replace(/\{(\w+)\}/g, (_, name: string) =>
		String(values[name] ?? `{${name}}`),
	);
}

export function getLocale(): Locale {
	return currentLocale;
}

export function t(
	key: string,
	values: Record<string, string | number> = {},
): string {
	const value = resolve(locales[currentLocale], key) ?? resolve(en, key);
	return typeof value === "string" ? interpolate(value, values) : key;
}

export function tp(
	key: string,
	count: number,
	values: Record<string, string | number> = {},
): string {
	const category = new Intl.PluralRules(currentLocale).select(count);
	const localized = resolve(locales[currentLocale], key);
	const fallback = resolve(en, key);
	const variants =
		(localized && typeof localized === "object" ? localized : undefined) ||
		(fallback && typeof fallback === "object" ? fallback : undefined);
	const template = variants?.[category] || variants?.other;
	return typeof template === "string"
		? interpolate(template, { ...values, count })
		: key;
}

export function setLocale(locale: Locale): void {
	if (!supportedLocales.includes(locale)) return;
	currentLocale = locale;
	document.documentElement.lang = locale;
	saveLocale(locale);
	translateDocument();
}

type Binding = {
	selector: string;
	key: string;
	attribute?: "aria-label" | "placeholder" | "title";
};

const bindings: Binding[] = [
	{ selector: ".skip-link", key: "app.skipToContent" },
	{
		selector: "#consoleMenuButton",
		key: "app.openSectionNavigation",
		attribute: "aria-label",
	},
	{
		selector: "#consoleMenuButton",
		key: "app.openSectionNavigation",
		attribute: "title",
	},
	{ selector: ".brand", key: "app.home", attribute: "aria-label" },
	{
		selector: "#headerMenuButton",
		key: "app.workspaceOptions",
		attribute: "aria-label",
	},
	{
		selector: "#headerMenuButton",
		key: "app.workspaceOptions",
		attribute: "title",
	},
	{ selector: "#refreshButton", key: "common.refresh" },
	{ selector: "#languageLabel", key: "query.language" },
	{ selector: "#sideNav", key: "nav.ariaLabel", attribute: "aria-label" },
	{ selector: "#logsNavButton span", key: "nav.logsExplorer" },
	{ selector: "#logsNavButton", key: "nav.logsExplorer", attribute: "title" },
	{ selector: "#timelineNavButton span", key: "nav.timeline" },
	{ selector: "#timelineNavButton", key: "nav.timeline", attribute: "title" },
	{ selector: "#fieldsNavButton span", key: "nav.fields" },
	{ selector: "#fieldsNavButton", key: "nav.fields", attribute: "title" },
	{ selector: ".page-heading h1", key: "nav.logsExplorer" },
	{ selector: ".page-heading p", key: "app.searchDescription" },
	{ selector: "#shareButton", key: "common.shareLink" },
	{ selector: ".query-panel .section-kicker", key: "query.filtersKicker" },
	{ selector: "#query-title", key: "query.searchLogs" },
	{ selector: ".query-shortcut", key: "query.advancedShortcutLong" },
	{ selector: "#clearQueryButton", key: "query.resetFilters" },
	{ selector: "#runQueryButton > span:nth-of-type(2)", key: "query.runQuery" },
	{ selector: ".search-all-fields .sr-only", key: "query.searchLogs" },
	{
		selector: "#searchAllFields",
		key: "query.searchPlaceholder",
		attribute: "placeholder",
	},
	{ selector: ".search-hint", key: "query.searchHint" },
	{
		selector: ".refine-row",
		key: "query.queryFilters",
		attribute: "aria-label",
	},
	{ selector: "#containerFilter + span", key: "filters.container" },
	{
		selector: ".refine-row label.filter-control:nth-of-type(1) > span",
		key: "filters.container",
	},
	{
		selector: ".refine-row label.filter-control:nth-of-type(2) > span",
		key: "filters.stream",
	},
	{
		selector: ".refine-row label.filter-control:nth-of-type(3) > span",
		key: "filters.severity",
	},
	{
		selector: ".refine-row label.filter-control:nth-of-type(4) > span",
		key: "filters.time",
	},
	{
		selector: "#containerFilter",
		key: "filters.container",
		attribute: "aria-label",
	},
	{ selector: "#streamFilter", key: "filters.stream", attribute: "aria-label" },
	{
		selector: "#severityFilter",
		key: "filters.severity",
		attribute: "aria-label",
	},
	{ selector: "#rangeFilter", key: "filters.time", attribute: "aria-label" },
	{ selector: "#customFromInput", key: "common.from", attribute: "aria-label" },
	{ selector: "#customToInput", key: "common.to", attribute: "aria-label" },
	{
		selector: "#customRangeEditor label:nth-child(1) > span",
		key: "common.from",
	},
	{
		selector: "#customRangeEditor label:nth-child(2) > span",
		key: "common.to",
	},
	{ selector: "#applyCustomRangeButton", key: "common.apply" },
	{ selector: "#clearCustomRangeButton", key: "common.clear" },
	{ selector: "#streamButton", key: "results.liveStream", attribute: "title" },
	{ selector: ".query-editor-heading strong", key: "query.advancedQuery" },
	{ selector: ".query-editor-heading span", key: "query.advancedShortcut" },
	{ selector: "label[for=queryInput]", key: "query.advancedLoggingQuery" },
	{
		selector: "#querySuggestions",
		key: "query.queryFilters",
		attribute: "aria-label",
	},
	{ selector: ".query-combination > span", key: "query.combinedWithBasic" },
	{
		selector: ".query-help > span:nth-of-type(2)",
		key: "query.basicFiltersHint",
	},
	{ selector: "#queryHelpButton", key: "query.querySyntax" },
	{
		selector: "#queryHelpPopover",
		key: "query.querySyntaxTitle",
		attribute: "aria-label",
	},
	{ selector: "#queryHelpTitle", key: "query.querySyntaxTitle" },
	{
		selector: "#queryHelpPopover p:first-of-type",
		key: "query.querySyntaxDescription",
	},
	{ selector: "#queryHelpPopover p:last-of-type", key: "query.combineClauses" },
	{ selector: "#errorDetails summary", key: "common.viewDetails" },
	{ selector: "#errorDismiss", key: "common.dismiss" },
	{ selector: "#fields .panel-heading h2", key: "fields.title" },
	{ selector: "#timeline .panel-heading h2", key: "timeline.title" },
	{
		selector: "#timelineZoomOut",
		key: "timeline.zoomOut",
		attribute: "aria-label",
	},
	{ selector: "#timelineZoomOut", key: "timeline.zoomOut", attribute: "title" },
	{
		selector: "#timelineZoomIn",
		key: "timeline.zoomIn",
		attribute: "aria-label",
	},
	{ selector: "#timelineZoomIn", key: "timeline.zoomIn", attribute: "title" },
	{
		selector: "#timelineToggle",
		key: "timeline.collapse",
		attribute: "aria-label",
	},
	{ selector: "#timelineToggle", key: "timeline.collapse", attribute: "title" },
	{
		selector: "#timelineLegend",
		key: "timeline.severityLegend",
		attribute: "aria-label",
	},
	{ selector: "#results-title", key: "results.title" },
	{ selector: "#approximateBadge", key: "results.partial" },
	{ selector: ".results-table-head span:nth-child(1)", key: "results.time" },
	{
		selector: ".results-table-head span:nth-child(2)",
		key: "results.severity",
	},
	{
		selector: ".results-table-head span:nth-child(3)",
		key: "results.resource",
	},
	{ selector: ".results-table-head span:nth-child(4)", key: "results.summary" },
	{ selector: "#wrapButton", key: "results.wrapLines" },
	{
		selector: "#sortFilter",
		key: "results.sortResults",
		attribute: "aria-label",
	},
	{
		selector: "#closeDetailButton",
		key: "detail.close",
		attribute: "aria-label",
	},
	{ selector: "#closeDetailButton", key: "detail.close", attribute: "title" },
	{ selector: ".detail-drawer .section-kicker", key: "detail.kicker" },
	{ selector: "#detailTitle", key: "detail.entryDetails" },
	{ selector: ".page-footer > span:first-child", key: "app.carolineForDocker" },
];

function setBinding(binding: Binding): void {
	const element = document.querySelector<HTMLElement>(binding.selector);
	if (!element) return;
	if (binding.attribute) {
		element.setAttribute(binding.attribute, t(binding.key));
	} else {
		element.textContent = t(binding.key);
	}
}

function setShortcut(
	selector: string,
	key: string,
	values: Record<string, string>,
): void {
	const element = document.querySelector<HTMLElement>(selector);
	if (!element) return;
	const html = t(key, values);
	element.innerHTML = html;
}

export function translateDocument(): void {
	document.documentElement.lang = currentLocale;
	document.title = t("app.title");
	document.querySelectorAll<HTMLElement>("[data-i18n]").forEach((element) => {
		const key = element.dataset.i18n;
		if (key) element.textContent = t(key);
	});
	document
		.querySelectorAll<HTMLElement>("[data-i18n-aria-label]")
		.forEach((element) => {
			const key = element.dataset.i18nAriaLabel;
			if (key) element.setAttribute("aria-label", t(key));
		});
	document
		.querySelectorAll<HTMLElement>("[data-i18n-placeholder]")
		.forEach((element) => {
			const key = element.dataset.i18nPlaceholder;
			if (key) element.setAttribute("placeholder", t(key));
		});
	bindings.forEach(setBinding);
	setShortcut(".query-shortcut", "query.advancedShortcutLong", {
		key: "<kbd>Ctrl</kbd>",
		enter: "<kbd>Enter</kbd>",
	});
	setShortcut(".query-editor-heading span", "query.advancedShortcut", {
		key: "<kbd>Ctrl</kbd>",
		enter: "<kbd>Enter</kbd>",
	});
	setShortcut(".search-hint", "query.searchHint", { key: "<kbd>/</kbd>" });
	setShortcut(".query-help > span:nth-of-type(2)", "query.basicFiltersHint", {
		key: "<kbd>Ctrl</kbd>",
		enter: "<kbd>Enter</kbd>",
	});
	setShortcut("#queryHelpPopover p:last-of-type", "query.combineClauses", {
		and: "<code>AND</code>",
		or: "<code>OR</code>",
	});
	const initialContainer = document.querySelector<HTMLSelectElement>(
		"#containerFilter option[value='']",
	);
	if (initialContainer)
		initialContainer.textContent = t("filters.allContainers");
	const initialStream = document.querySelector<HTMLSelectElement>(
		"#streamFilter option[value='']",
	);
	if (initialStream) initialStream.textContent = t("filters.allStreams");
	const initialSeverity = document.querySelector<HTMLSelectElement>(
		"#severityFilter option[value='']",
	);
	if (initialSeverity) initialSeverity.textContent = t("filters.allSeverities");
	const severityOptions: Record<string, string> = {
		ERROR: "filters.error",
		WARNING: "filters.warning",
		INFO: "filters.info",
		DEBUG: "filters.debug",
	};
	Object.entries(severityOptions).forEach(([value, key]) => {
		const option = document.querySelector<HTMLOptionElement>(
			`#severityFilter option[value="${value}"]`,
		);
		if (option) option.textContent = t(key);
	});
	const durationOptions: Record<string, string> = {
		"5m": "durations.fiveMinutes",
		"15m": "durations.fifteenMinutes",
		"1h": "durations.oneHour",
		"6h": "durations.sixHours",
		"24h": "durations.twentyFourHours",
		"7d": "durations.sevenDays",
		custom: "filters.selectedInterval",
	};
	Object.entries(durationOptions).forEach(([value, key]) => {
		const option = document.querySelector<HTMLOptionElement>(
			`#rangeFilter option[value="${value}"]`,
		);
		if (option) option.textContent = t(key);
	});
}

export {
	isSupportedLocale,
	supportedLocales,
	type Locale,
} from "./locale";
