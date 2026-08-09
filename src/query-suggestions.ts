import { $$, $, escapeHTML } from "./dom.js";
import { state } from "./state.js";
import type { QuerySuggestion } from "./types.js";

const queryFields = [
	{ name: "severity", detail: "Log severity" },
	{ name: "resource.labels.container_name", detail: "Container name" },
	{ name: "resource.labels.container_id", detail: "Container ID" },
	{ name: "resource.labels.image", detail: "Container image" },
	{ name: "resource.type", detail: "Resource type" },
	{ name: "logName", detail: "Log name" },
	{ name: "stream", detail: "stdout or stderr" },
	{ name: "textPayload", detail: "Plain-text payload" },
	{ name: "jsonPayload.field", detail: "JSON payload field" },
	{ name: "timestamp", detail: "Log timestamp" },
];
const queryOperators = ["=", "!=", ":", ">=", "<=", ">", "<"];
let visibleQuerySuggestions: QuerySuggestion[] = [];
let querySuggestionIndex = 0;

function queryToken(input: HTMLTextAreaElement): {
	line: string;
	lineStart: number;
	token: string;
	tokenStart: number;
} {
	const cursor = input.selectionStart;
	const beforeCursor = input.value.slice(0, cursor);
	const lineStart =
		Math.max(beforeCursor.lastIndexOf("\n"), beforeCursor.lastIndexOf("\r")) +
		1;
	const line = beforeCursor.slice(lineStart);
	const match = line.match(/([A-Za-z0-9_.-]*)$/);
	const token = match?.[1] || "";
	return {
		line,
		lineStart,
		token,
		tokenStart: cursor - token.length,
	};
}

function matchingSuggestions(
	items: QuerySuggestion[],
	token: string,
): QuerySuggestion[] {
	const normalized = token.toLowerCase();
	return items.filter((item) =>
		item.label.toLowerCase().startsWith(normalized),
	);
}

function valueSuggestions(
	field: string,
	token: string,
	start: number,
	end: number,
): QuerySuggestion[] {
	let values: string[] = [];
	switch (field) {
		case "severity":
			values = ["DEBUG", "INFO", "NOTICE", "WARNING", "ERROR", "CRITICAL"];
			break;
		case "stream":
			values = ["stdout", "stderr"];
			break;
		case "resource.type":
			values = ["docker_container"];
			break;
		case "resource.labels.container_name":
			values = state.containers.map((container) => container.name);
			break;
		case "resource.labels.container_id":
			values = state.containers.map((container) => container.id);
			break;
		case "resource.labels.image":
			values = state.containers.map((container) => container.image);
			break;
	}
	return matchingSuggestions(
		[...new Set(values)].map((value) => ({
			label: value,
			detail: `${field} value`,
			replacement: `"${value.replace(/"/g, '\\"')}"`,
			replaceStart: start,
			replaceEnd: end,
		})),
		token.replace(/^"/, "").replace(/"$/, ""),
	).slice(0, 8);
}

function operatorSuggestions(
	field: string,
	start: number,
	end: number,
): QuerySuggestion[] {
	return queryOperators.map((operator) => ({
		label: `${field} ${operator}`,
		detail: "Query operator",
		replacement: `${field} ${operator} `,
		replaceStart: start,
		replaceEnd: end,
	}));
}

function buildQuerySuggestions(input: HTMLTextAreaElement): QuerySuggestion[] {
	const { line, lineStart, token, tokenStart } = queryToken(input);
	const before = line.trimStart();
	const valueMatch = before.match(
		/^([A-Za-z0-9_.-]+)\s*(>=|<=|!=|=|:|>|<)\s*([A-Za-z0-9_.-]*)$/,
	);
	if (valueMatch) {
		const valueStart = lineStart + line.lastIndexOf(valueMatch[3]);
		return valueSuggestions(
			valueMatch[1],
			valueMatch[3],
			valueStart,
			input.selectionStart,
		);
	}

	const fieldMatch = before.match(/^([A-Za-z0-9_.-]+)\s*$/);
	if (fieldMatch) {
		const field = queryFields.find((item) => item.name === fieldMatch[1]);
		const fieldStart = lineStart + line.indexOf(fieldMatch[1]);
		if (field)
			return operatorSuggestions(field.name, fieldStart, input.selectionStart);
	}

	const fields = queryFields.map((field) => ({
		label: field.name,
		detail: field.detail,
		replacement: field.name,
		replaceStart: tokenStart,
		replaceEnd: input.selectionStart,
	}));
	const keywords: QuerySuggestion[] = [
		{
			label: "AND",
			detail: "Combine with the next clause",
			replacement: "AND ",
			replaceStart: tokenStart,
			replaceEnd: input.selectionStart,
		},
		{
			label: "OR",
			detail: "Match either clause",
			replacement: "OR ",
			replaceStart: tokenStart,
			replaceEnd: input.selectionStart,
		},
		{
			label: 'SEARCH("")',
			detail: "Search across log fields",
			replacement: 'SEARCH("")',
			cursorOffset: 8,
			replaceStart: tokenStart,
			replaceEnd: input.selectionStart,
		},
	];
	return [
		...matchingSuggestions(fields, token),
		...matchingSuggestions(keywords, token),
	].slice(0, 10);
}

export function closeQuerySuggestions(): void {
	visibleQuerySuggestions = [];
	querySuggestionIndex = 0;
	const suggestions = $("#querySuggestions");
	suggestions.setAttribute("hidden", "");
	$("#queryInput").setAttribute("aria-expanded", "false");
	$("#queryInput").removeAttribute("aria-activedescendant");
}

export function renderQuerySuggestions(): void {
	const input = $("#queryInput") as HTMLTextAreaElement;
	const suggestions = $("#querySuggestions");
	visibleQuerySuggestions = buildQuerySuggestions(input);
	if (!visibleQuerySuggestions.length) {
		closeQuerySuggestions();
		return;
	}
	querySuggestionIndex = Math.min(
		querySuggestionIndex,
		visibleQuerySuggestions.length - 1,
	);
	suggestions.innerHTML = visibleQuerySuggestions
		.map(
			(suggestion, index) =>
				`<button class="query-suggestion${index === querySuggestionIndex ? " active" : ""}" type="button" role="option" aria-selected="${index === querySuggestionIndex}" data-suggestion-index="${index}" id="query-suggestion-${index}"><span class="query-suggestion-label">${escapeHTML(suggestion.label)}</span><span class="query-suggestion-detail">${escapeHTML(suggestion.detail)}</span></button>`,
		)
		.join("");
	suggestions.removeAttribute("hidden");
	input.setAttribute("aria-expanded", "true");
	input.setAttribute(
		"aria-activedescendant",
		`query-suggestion-${querySuggestionIndex}`,
	);
	$$(".query-suggestion").forEach((button) =>
		button.addEventListener("mousedown", (event) => event.preventDefault()),
	);
	$$(".query-suggestion").forEach((button) =>
		button.addEventListener("click", () =>
			applyQuerySuggestion(Number(button.dataset.suggestionIndex)),
		),
	);
}

function applyQuerySuggestion(index: number): void {
	const suggestion = visibleQuerySuggestions[index];
	if (!suggestion) return;
	const input = $("#queryInput") as HTMLTextAreaElement;
	input.value =
		input.value.slice(0, suggestion.replaceStart) +
		suggestion.replacement +
		input.value.slice(suggestion.replaceEnd);
	const cursor =
		suggestion.replaceStart +
		(suggestion.cursorOffset ?? suggestion.replacement.length);
	input.setSelectionRange(cursor, cursor);
	state.draftQuery = input.value;
	input.focus();
	querySuggestionIndex = 0;
	renderQuerySuggestions();
}

export function handleQueryKeydown(event: KeyboardEvent): void {
	if (event.key === "ArrowDown" && visibleQuerySuggestions.length) {
		event.preventDefault();
		querySuggestionIndex =
			(querySuggestionIndex + 1) % visibleQuerySuggestions.length;
		renderQuerySuggestions();
		return;
	}
	if (event.key === "ArrowUp" && visibleQuerySuggestions.length) {
		event.preventDefault();
		querySuggestionIndex =
			(querySuggestionIndex - 1 + visibleQuerySuggestions.length) %
			visibleQuerySuggestions.length;
		renderQuerySuggestions();
		return;
	}
	if (event.key === "Tab" && visibleQuerySuggestions.length) {
		event.preventDefault();
		applyQuerySuggestion(querySuggestionIndex);
		return;
	}
	if (event.key === "Escape" && visibleQuerySuggestions.length) {
		event.preventDefault();
		event.stopPropagation();
		closeQuerySuggestions();
	}
}
