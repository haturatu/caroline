import {
	buildExplorerURL,
	fetchExplorer,
	fetchStatus,
	hydrateURL,
	syncURL,
} from "./api.js";
import { $$, $ } from "./dom.js";
import { errorText } from "./format.js";
import {
	closeQuerySuggestions,
	handleQueryKeydown,
	renderQuerySuggestions,
} from "./query-suggestions.js";
import {
	renderAll,
	renderDetail,
	renderEntries,
	renderFilters,
	renderLoading,
	renderResultsMeta,
	setRenderActions,
} from "./render.js";
import { state } from "./state.js";
import type { Theme } from "./types.js";

let refreshTimer: number | null = null;
let detailReturnFocusId: string | null = null;
let errorContextFocused = false;

function renderErrorBanner(): void {
	const banner = $("#errorBanner");
	const messages = [state.errors.status, state.errors.explorer].filter(Boolean);
	if (!messages.length) {
		banner.setAttribute("hidden", "");
		errorContextFocused = false;
		return;
	}
	const wasHidden = banner.hasAttribute("hidden");
	$("#errorMessage").textContent = messages.join(" · ");
	banner.removeAttribute("hidden");
	if (
		wasHidden &&
		!errorContextFocused &&
		document.activeElement === document.body
	) {
		$("#main-content").focus({ preventScroll: true });
		errorContextFocused = true;
	}
}

function showError(
	message: string,
	source: "status" | "explorer" = "explorer",
): void {
	state.errors[source] = message;
	renderErrorBanner();
}

function clearError(source?: "status" | "explorer"): void {
	if (source) state.errors[source] = "";
	else state.errors = { status: "", explorer: "" };
	renderErrorBanner();
}

function toast(message: string): void {
	const element = $("#toast");
	element.textContent = message;
	element.removeAttribute("hidden");
	window.setTimeout(() => element.setAttribute("hidden", ""), 2400);
}

function applyTheme(theme: Theme): void {
	state.theme = theme;
	document.documentElement.dataset.theme = theme;
	try {
		window.localStorage.setItem("caroline-theme", theme);
	} catch {
		// Local storage can be unavailable in private browsing contexts.
	}
	const themeColor = document.querySelector<HTMLMetaElement>("#themeColor");
	if (themeColor) themeColor.content = theme === "dark" ? "#202124" : "#f8fafd";
	$("#themeToggleButton").textContent =
		theme === "dark" ? "Use light theme" : "Use dark theme";
}

function loadSavedTheme(): Theme {
	try {
		return window.localStorage.getItem("caroline-theme") === "light"
			? "light"
			: "dark";
	} catch {
		return "dark";
	}
}

function closeHeaderMenus(): void {
	$("#headerMenu").setAttribute("hidden", "");
	$("#headerMenuButton").setAttribute("aria-expanded", "false");
}

function toggleMenu(menu: HTMLElement, trigger: HTMLElement): void {
	const willOpen = menu.hasAttribute("hidden");
	closeHeaderMenus();
	if (willOpen) {
		menu.removeAttribute("hidden");
		trigger.setAttribute("aria-expanded", "true");
	}
}

function focusGlobalSearch(): void {
	const input = $("#searchAllFields");
	input.focus();
	if (window.matchMedia("(max-width: 800px)").matches)
		input.scrollIntoView({ behavior: "smooth", block: "center" });
}

function setActiveNavigation(id: string): void {
	$$<HTMLButtonElement>(".side-nav-link").forEach((button) => {
		const active = button.id === id;
		button.classList.toggle("active", active);
		if (active) button.setAttribute("aria-current", "page");
		else button.removeAttribute("aria-current");
	});
}

function focusSection(section: "main-content" | "timeline" | "fields"): void {
	if (section === "fields") {
		state.fieldsHidden = false;
		renderAll();
	}
	if (section === "timeline") {
		state.timelineHidden = false;
		renderAll();
	}
	const target = $(`#${section}`);
	target.scrollIntoView({ behavior: "smooth", block: "center" });
	requestAnimationFrame(() => target.focus({ preventScroll: true }));
	setActiveNavigation(
		section === "main-content"
			? "logsNavButton"
			: section === "timeline"
				? "timelineNavButton"
				: "fieldsNavButton",
	);
}

async function loadStatus(): Promise<void> {
	try {
		const status = await fetchStatus();
		$("#connectionDot").className = status.connected ? "connected" : "offline";
		$("#connectionText").textContent = status.connected
			? "Connected"
			: "Unavailable";
		$("#sideEngineStatus").textContent = status.connected
			? "Docker Connected"
			: "Docker Unavailable";
		$("#sideEngineVersion").textContent = status.connected
			? `Engine ${status.dockerVersion || "Unknown"}`
			: "Mount Docker socket";
		if (status.connected) {
			clearError("status");
		} else {
			showError(
				"Docker is unavailable. Start Docker and mount /var/run/docker.sock into Caroline.",
				"status",
			);
		}
	} catch (error) {
		$("#connectionDot").className = "offline";
		$("#connectionText").textContent = "Server Error";
		showError(errorText(error), "status");
	}
}

async function loadExplorer(append = false): Promise<void> {
	if (state.loading) return;
	state.loading = true;
	if (!append && !state.response) renderLoading();
	if (append) renderResultsMeta();
	try {
		const response = await fetchExplorer();
		state.response = response;
		state.containers = response.containers || [];
		const incoming = response.entries || [];
		if (append) {
			const existingIds = new Set(state.entries.map((entry) => entry.insertId));
			state.entries = [
				...state.entries,
				...incoming.filter((entry) => {
					if (existingIds.has(entry.insertId)) return false;
					existingIds.add(entry.insertId);
					return true;
				}),
			];
			state.pageToken = response.nextPageToken || "";
		} else {
			state.entries = incoming;
		}
		if (
			state.selectedId &&
			!state.entries.some((entry) => entry.insertId === state.selectedId)
		) {
			state.selectedId = null;
			detailReturnFocusId = null;
			setDrawerOpen(false);
		}
		clearError("explorer");
		renderAll();
		if (response.errors?.length)
			showError(
				`Some containers could not be read: ${response.errors.join(" · ")}`,
				"explorer",
			);
	} catch (error) {
		if (!append && !state.response) {
			state.entries = [];
			state.loading = false;
			renderAll();
		}
		showError(errorText(error), "explorer");
	} finally {
		state.loading = false;
		renderResultsMeta();
	}
}

function setLive(value: boolean): void {
	state.live = value;
	syncURL();
	if (refreshTimer !== null) window.clearInterval(refreshTimer);
	refreshTimer = value
		? window.setInterval(() => {
				if (!state.pageToken) void loadExplorer();
				void loadStatus();
			}, 5000)
		: null;
	renderAll();
}

function runQuery(): void {
	state.query = ($("#queryInput") as HTMLTextAreaElement).value.trim();
	state.draftQuery = state.query;
	state.searchText = ($("#searchAllFields") as HTMLInputElement).value.trim();
	state.pageToken = "";
	closeQuerySuggestions();
	syncURL();
	void loadExplorer();
}

function shiftTimelineRange(step: number): void {
	const ranges = ["5m", "15m", "1h", "6h", "24h", "7d"];
	const current = Math.max(0, ranges.indexOf(state.duration));
	const next = Math.min(ranges.length - 1, Math.max(0, current + step));
	if (next === current) return;
	state.duration = ranges[next];
	state.timeFrom = "";
	state.timeTo = "";
	state.pageToken = "";
	($("#rangeFilter") as HTMLSelectElement).value = state.duration;
	syncURL();
	void loadExplorer();
}

function setDrawerOpen(open: boolean): void {
	const appShell = $("#appShell") as HTMLElement & { inert: boolean };
	appShell.inert = open;
	document.body.classList.toggle("drawer-open", open);
}

function toDateTimeLocal(value: string): string {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return "";
	const pad = (part: number) => String(part).padStart(2, "0");
	return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function fromDateTimeLocal(value: string): string {
	const date = new Date(value);
	return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function initializeCustomRange(): void {
	if (state.timeFrom && state.timeTo) return;
	const to = new Date();
	const from = new Date(to.getTime() - 5 * 60 * 1000);
	state.timeFrom = from.toISOString();
	state.timeTo = to.toISOString();
}

function applyCustomRange(): void {
	const from = fromDateTimeLocal(
		($("#customFromInput") as HTMLInputElement).value,
	);
	const to = fromDateTimeLocal(
		($("#customToInput") as HTMLInputElement).value,
	);
	if (!from || !to || new Date(from).getTime() >= new Date(to).getTime()) {
		showError("The custom time range must include a valid start before its end.");
		return;
	}
	state.timeFrom = from;
	state.timeTo = to;
	state.pageToken = "";
	clearError("explorer");
	syncURL();
	renderAll();
	void loadExplorer();
}

function clearCustomRange(): void {
	state.timeFrom = "";
	state.timeTo = "";
	state.pageToken = "";
	clearError("explorer");
	syncURL();
	renderAll();
	void loadExplorer();
}

function openDetail(entryId: string): void {
	const drawerWasHidden = $("#detailDrawer").hasAttribute("hidden");
	detailReturnFocusId = entryId;
	state.selectedId = entryId;
	setDrawerOpen(true);
	renderEntries();
	renderDetail();
	if (drawerWasHidden) {
		const focusCloseButton = () => $("#closeDetailButton").focus();
		focusCloseButton();
		requestAnimationFrame(focusCloseButton);
	}
}

function closeDetail(): void {
	setDrawerOpen(false);
	state.selectedId = null;
	renderDetail();
	renderEntries();
	const returnFocusId = detailReturnFocusId;
	detailReturnFocusId = null;
	if (returnFocusId)
		$$<HTMLElement>(".entry-row")
			.find((row) => row.getAttribute("data-entry-id") === returnFocusId)
			?.focus();
}

function setupRenderActions(): void {
	setRenderActions({
		onToast: toast,
		onEntrySelect: openDetail,
		onFieldFilter: (field, value) => {
			state.query = `${state.query}${state.query ? "\n" : ""}${field} = "${value.replace(/"/g, '\\"')}"`;
			state.draftQuery = state.query;
			state.showQuery = true;
			state.pageToken = "";
			syncURL();
			renderFilters();
			void loadExplorer();
		},
		onTimelineSelect: (start, end) => {
			state.timeFrom = start;
			state.timeTo = end;
			state.pageToken = "";
			syncURL();
			renderAll();
			void loadExplorer();
		},
	});
}

function setupEvents(): void {
	$("#consoleMenuButton").addEventListener("click", () => {
		state.navExpanded = !state.navExpanded;
		document.body.classList.toggle("nav-expanded", state.navExpanded);
		$("#consoleMenuButton").setAttribute(
			"aria-expanded",
			String(state.navExpanded),
		);
	});
	$("#globalSearchButton").addEventListener("click", focusGlobalSearch);
	$("#headerMenuButton").addEventListener("click", () =>
		toggleMenu($("#headerMenu"), $("#headerMenuButton")),
	);
	$("#themeToggleButton").addEventListener("click", () => {
		applyTheme(state.theme === "dark" ? "light" : "dark");
		closeHeaderMenus();
	});
	$("#refreshButton").addEventListener("click", () => {
		state.pageToken = "";
		closeHeaderMenus();
		void loadStatus();
		void loadExplorer();
	});
	$("#logsNavButton").addEventListener("click", () =>
		focusSection("main-content"),
	);
	$("#timelineNavButton").addEventListener("click", () =>
		focusSection("timeline"),
	);
	$("#fieldsNavButton").addEventListener("click", () => focusSection("fields"));
	$("#runQueryButton").addEventListener("click", runQuery);
	$("#clearQueryButton").addEventListener("click", () => {
		state.query = state.draftQuery = "";
		state.searchText = "";
		state.timeFrom = "";
		state.timeTo = "";
		state.pageToken = "";
		syncURL();
		renderFilters();
		void loadExplorer();
	});
	$("#searchAllFields").addEventListener("input", (event: Event) => {
		state.searchText = (event.target as HTMLInputElement).value;
	});
	$("#showQueryButton").addEventListener("click", () => {
		state.showQuery = !state.showQuery;
		if (!state.showQuery) closeQuerySuggestions();
		renderFilters();
	});
	$("#queryInput").addEventListener("input", () => {
		state.draftQuery = ($("#queryInput") as HTMLTextAreaElement).value;
		renderQuerySuggestions();
	});
	$("#queryInput").addEventListener("focus", renderQuerySuggestions);
	$("#queryInput").addEventListener("click", renderQuerySuggestions);
	$("#queryInput").addEventListener("keydown", (event) => {
		handleQueryKeydown(event);
		if (event.defaultPrevented) return;
		if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
			event.preventDefault();
			runQuery();
		}
	});
	document.addEventListener("click", (event: MouseEvent) => {
		const target = event.target as Element;
		if (!target.closest(".query-editor")) closeQuerySuggestions();
		if (!target.closest(".global-header")) closeHeaderMenus();
	});
	$("#containerFilter").addEventListener("change", (event: Event) => {
		state.container = (event.target as HTMLSelectElement).value;
		state.pageToken = "";
		syncURL();
		void loadExplorer();
	});
	$("#streamFilter").addEventListener("change", (event: Event) => {
		state.stream = (event.target as HTMLSelectElement).value;
		state.pageToken = "";
		syncURL();
		void loadExplorer();
	});
	$("#severityFilter").addEventListener("change", (event: Event) => {
		state.severity = (event.target as HTMLSelectElement)
			.value as typeof state.severity;
		state.pageToken = "";
		syncURL();
		void loadExplorer();
	});
	$("#rangeFilter").addEventListener("change", (event: Event) => {
		const value = (event.target as HTMLSelectElement).value;
		if (value === "custom") {
			initializeCustomRange();
			renderAll();
			return;
		}
		state.duration = value;
		state.timeFrom = "";
		state.timeTo = "";
		state.pageToken = "";
		syncURL();
		void loadExplorer();
	});
	$("#sortFilter").addEventListener("change", (event: Event) => {
		state.sort = (event.target as HTMLSelectElement).value as typeof state.sort;
		state.pageToken = "";
		syncURL();
		void loadExplorer();
	});
	$("#streamButton").addEventListener("click", () => setLive(!state.live));
	$("#wrapButton").addEventListener("click", () => {
		state.wrap = !state.wrap;
		renderEntries();
	});
	$("#nextPageButton").addEventListener("click", () => {
		if (state.response?.nextPageToken) {
			state.pageToken = state.response.nextPageToken;
			void loadExplorer(true);
		}
	});
	$("#applyCustomRangeButton").addEventListener("click", applyCustomRange);
	$("#clearCustomRangeButton").addEventListener("click", clearCustomRange);
	$("#closeDetailButton").addEventListener("click", closeDetail);
	$("#errorDismiss").addEventListener("click", () => clearError());
	$("#shareButton").addEventListener("click", () => {
		const copy = navigator.clipboard?.writeText(window.location.href);
		if (copy) void copy.then(() => toast("Query link copied."));
		else toast("Copy the current URL from the address bar.");
	});
	$("#queryHelpButton").addEventListener("click", () =>
		toast('Use field = value, field >= value, SEARCH("text"), AND, and OR.'),
	);
	$("#fieldsToggle").addEventListener("click", () => {
		state.fieldsHidden = !state.fieldsHidden;
		renderAll();
	});
	$("#timelineToggle").addEventListener("click", () => {
		state.timelineHidden = !state.timelineHidden;
		renderAll();
	});
	$("#timelineZoomOut").addEventListener("click", () => shiftTimelineRange(1));
	$("#timelineZoomIn").addEventListener("click", () => shiftTimelineRange(-1));
	window.addEventListener("popstate", () => {
		hydrateURL();
		state.pageToken = "";
		void loadExplorer();
		setLive(state.live);
	});
	document.addEventListener("keydown", (event: KeyboardEvent) => {
		const activeTag = document.activeElement?.tagName;
		if (
			event.key === "/" &&
			activeTag !== "INPUT" &&
			activeTag !== "TEXTAREA" &&
			activeTag !== "SELECT"
		) {
			event.preventDefault();
			focusGlobalSearch();
		}
		if (event.key === "Escape") {
			if (!$("#detailDrawer").hasAttribute("hidden")) closeDetail();
			else closeHeaderMenus();
		}
		if (event.key === "Tab" && !$("#detailDrawer").hasAttribute("hidden")) {
			const focusable = $$<HTMLElement>(
				"#detailDrawer button, #detailDrawer [href], #detailDrawer input, #detailDrawer textarea",
			).filter((element) => !element.hasAttribute("disabled"));
			if (!focusable.length) return;
			const first = focusable[0];
			const last = focusable[focusable.length - 1];
			if (event.shiftKey && document.activeElement === first) {
				event.preventDefault();
				last.focus();
			} else if (!event.shiftKey && document.activeElement === last) {
				event.preventDefault();
				first.focus();
			}
		}
	});
}

hydrateURL();
syncURL();
setupRenderActions();
setupEvents();
applyTheme(loadSavedTheme());
setLive(state.live);
void loadStatus();
void loadExplorer();
