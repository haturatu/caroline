import { fetchExplorer, fetchStatus } from "../features/explorer/api";
import { hydrateURL, syncURL } from "./url-state";
import { closeTail, startTail } from "../features/streaming/controller";
import { setupTimelineResolution } from "../features/timeline/resolution";
import { $$, $, escapeHTML } from "../shared/dom/selectors";
import { copyText } from "../shared/dom/clipboard";
import { errorText } from "../shared/format";
import {
	getLocale,
	isSupportedLocale,
	setLocale,
	t,
	tp,
} from "../shared/i18n/index";
import {
	closeQuerySuggestions,
	handleQueryKeydown,
	renderQuerySuggestions,
} from "../features/query-editor/suggestions";
import {
	renderAll,
	renderLoading,
	renderResultsMeta,
} from "../features/explorer/render";
import { setRenderActions } from "../features/explorer/actions";
import { renderFilters } from "../features/filters/render";
import { renderEntries } from "../features/logs/render";
import { renderDetail } from "../features/logs/detail";
import { state } from "./state";
import type { Theme } from "../shared/types";

let statusTimer: number | null = null;
let searchTimer: number | null = null;
let reloadRequested = false;
let detailReturnFocusId: string | null = null;
let navReturnFocus: HTMLElement | null = null;
let fieldsReturnFocus: HTMLElement | null = null;
let errorContextFocused = false;
let initialLoadingTimer: number | null = null;
let initialLoadingVisibleAt = 0;
const loadingShowDelay = 200;
const loadingMinimumDuration = 350;

function renderErrorBanner(): void {
	const banner = $("#errorBanner");
	const messages = [state.errors.status, state.errors.explorer].filter(Boolean);
	if (!messages.length) {
		banner.setAttribute("hidden", "");
		errorContextFocused = false;
		return;
	}
	const wasHidden = banner.hasAttribute("hidden");
	$("#errorMessage").textContent = state.errorDetails.length
		? tp("errors.containerRead", state.errorDetails.length)
		: messages.join(" · ");
	const detailsToggle = $("#errorDetailsToggle");
	const details = $("#errorDetails");
	if (state.errorDetails.length) {
		detailsToggle.removeAttribute("hidden");
		details.innerHTML = `<ul>${state.errorDetails.map((detail) => `<li>${escapeHTML(detail)}</li>`).join("")}</ul>`;
	} else {
		detailsToggle.setAttribute("hidden", "");
		detailsToggle.setAttribute("aria-expanded", "false");
		details.setAttribute("hidden", "");
		details.innerHTML = "";
	}
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
	if (!source || source === "explorer") state.errorDetails = [];
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
		theme === "dark" ? t("theme.useLight") : t("theme.useDark");
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

function closeHeaderMenus(returnFocus = false): void {
	const wasOpen = !$("#headerMenu").hasAttribute("hidden");
	$("#headerMenu").setAttribute("hidden", "");
	$("#headerMenuButton").setAttribute("aria-expanded", "false");
	if (returnFocus && wasOpen)
		$("#headerMenuButton").focus({ preventScroll: true });
}

function toggleMenu(menu: HTMLElement, trigger: HTMLElement): void {
	const willOpen = menu.hasAttribute("hidden");
	closeHeaderMenus();
	if (willOpen) {
		menu.removeAttribute("hidden");
		trigger.setAttribute("aria-expanded", "true");
		requestAnimationFrame(() =>
			menu.querySelector<HTMLButtonElement>("button")?.focus(),
		);
	}
}

function focusGlobalSearch(): void {
	const input = $("#searchAllFields");
	input.focus();
	if (window.matchMedia("(max-width: 800px)").matches)
		input.scrollIntoView({ behavior: "smooth", block: "center" });
}

function isMobileViewport(): boolean {
	return window.matchMedia("(max-width: 800px)").matches;
}

function syncMobileFieldsOverlay(): void {
	const fieldsOpen = isMobileViewport() && !state.fieldsHidden;
	document.body.classList.toggle("fields-open", fieldsOpen);
	if (isMobileViewport())
		setActiveNavigation(fieldsOpen ? "fieldsNavButton" : "logsNavButton");
	if (fieldsOpen) {
		if (!fieldsReturnFocus)
			fieldsReturnFocus = $("#consoleMenuButton") as HTMLElement;
		$("#mobileNavBackdrop").removeAttribute("hidden");
	} else if (!state.navExpanded) {
		$("#mobileNavBackdrop").setAttribute("hidden", "");
		fieldsReturnFocus = null;
	}
}

function closeMobileOverlay(): void {
	const fieldsWereOpen = document.body.classList.contains("fields-open");
	const returnFocus = fieldsWereOpen
		? fieldsReturnFocus || ($("#consoleMenuButton") as HTMLElement)
		: navReturnFocus;
	state.navExpanded = false;
	document.body.classList.remove("nav-expanded", "fields-open");
	$("#consoleMenuButton").setAttribute("aria-expanded", "false");
	$("#mobileNavBackdrop").setAttribute("hidden", "");
	if (fieldsWereOpen) {
		state.fieldsHidden = true;
		syncURL();
		renderAll();
		setActiveNavigation("logsNavButton");
	}
	navReturnFocus = null;
	fieldsReturnFocus = null;
	if (returnFocus?.isConnected) returnFocus.focus({ preventScroll: true });
}

function toggleMobileNav(): void {
	if (document.body.classList.contains("fields-open")) {
		closeMobileOverlay();
		return;
	}
	if (state.navExpanded) {
		closeMobileOverlay();
		return;
	}
	navReturnFocus =
		document.activeElement instanceof HTMLElement
			? (document.activeElement as HTMLElement)
			: ($("#consoleMenuButton") as HTMLElement);
	state.navExpanded = true;
	document.body.classList.add("nav-expanded");
	document.body.classList.remove("fields-open");
	$("#mobileNavBackdrop").removeAttribute("hidden");
	$("#consoleMenuButton").setAttribute("aria-expanded", "true");
	requestAnimationFrame(() => $("#logsNavButton").focus());
}

function setActiveNavigation(id: string): void {
	$$<HTMLButtonElement>(".side-nav-link").forEach((button) => {
		const active = button.id === id;
		button.classList.toggle("active", active);
		if (active) button.setAttribute("aria-current", "location");
		else button.removeAttribute("aria-current");
	});
}

function focusSection(section: "main-content" | "timeline" | "fields"): void {
	const mobile = isMobileViewport();
	if (section === "fields") {
		state.fieldsHidden = false;
		if (mobile) fieldsReturnFocus = $("#consoleMenuButton") as HTMLElement;
		renderAll();
	}
	if (section === "timeline") {
		state.timelineHidden = false;
		renderAll();
	}
	syncURL();
	if (mobile) {
		closeMobileOverlay();
		if (section === "fields") {
			document.body.classList.add("fields-open");
			$("#mobileNavBackdrop").removeAttribute("hidden");
		}
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

function scheduleInitialLoading(): void {
	initialLoadingVisibleAt = 0;
	initialLoadingTimer = window.setTimeout(() => {
		initialLoadingTimer = null;
		if (state.loading && !state.response) {
			renderLoading();
			initialLoadingVisibleAt = performance.now();
		}
	}, loadingShowDelay);
}

async function settleInitialLoading(): Promise<void> {
	if (initialLoadingTimer !== null) {
		window.clearTimeout(initialLoadingTimer);
		initialLoadingTimer = null;
	}
	if (initialLoadingVisibleAt > 0) {
		const elapsed = performance.now() - initialLoadingVisibleAt;
		const remaining = Math.max(0, loadingMinimumDuration - elapsed);
		if (remaining > 0)
			await new Promise<void>((resolve) =>
				window.setTimeout(resolve, remaining),
			);
	}
	initialLoadingVisibleAt = 0;
}

async function loadStatus(): Promise<void> {
	try {
		const status = await fetchStatus();
		$("#sideEngineStatus").textContent = status.connected
			? t("status.connected")
			: t("status.unavailable");
		$("#sideEngineVersion").textContent = status.connected
			? t("status.engineVersion", {
					version: status.dockerVersion || t("status.unknownVersion"),
				})
			: t("status.mountDockerSocket");
		if (status.connected) {
			clearError("status");
		} else {
			showError(t("status.connectionError"), "status");
		}
	} catch (error) {
		showError(errorText(error), "status");
	}
}

async function loadExplorer(append = false): Promise<void> {
	if (!append) closeTail();
	if (state.loading) {
		reloadRequested = true;
		return;
	}
	state.loading = true;
	state.errorDetails = [];
	const requestedPageToken = state.pageToken;
	const showInitialLoading = !append && !state.response;
	if (showInitialLoading) scheduleInitialLoading();
	renderResultsMeta();
	try {
		const response = await fetchExplorer();
		state.response = response;
		state.lastUpdated = response.generatedAt;
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
		} else {
			state.entries = incoming;
		}
		state.pageToken =
			append || requestedPageToken ? response.nextPageToken || "" : "";
		if (append || requestedPageToken) syncURL();
		if (
			state.selectedId &&
			!state.entries.some((entry) => entry.insertId === state.selectedId)
		) {
			state.selectedId = null;
			detailReturnFocusId = null;
			setDrawerOpen(false);
		}
		await settleInitialLoading();
		state.loading = false;
		clearError("explorer");
		renderAll();
		state.errorDetails = response.errors || [];
		if (state.errorDetails.length)
			showError(t("errors.someContainers"), "explorer");
		if (!append)
			startTail(response.generatedAt, (message) =>
				showError(message, "explorer"),
			);
	} catch (error) {
		await settleInitialLoading();
		if (!append && !state.response) {
			state.entries = [];
			state.loading = false;
			renderAll();
		}
		showError(errorText(error), "explorer");
	} finally {
		state.loading = false;
		renderResultsMeta();
		if (reloadRequested) {
			reloadRequested = false;
			void loadExplorer();
		}
	}
}

function setLive(value: boolean): void {
	state.live = value;
	syncURL();
	if (statusTimer === null) {
		statusTimer = window.setInterval(() => void loadStatus(), 30000);
	}
	if (value && state.response)
		startTail(state.response.generatedAt, (message) =>
			showError(message, "explorer"),
		);
	if (!value) closeTail();
	renderAll();
}

function runQuery(): void {
	state.query = ($("#queryInput") as HTMLTextAreaElement).value.trim();
	state.draftQuery = state.query;
	state.searchText = ($("#searchAllFields") as HTMLInputElement).value.trim();
	if (searchTimer !== null) window.clearTimeout(searchTimer);
	state.pageToken = "";
	closeQuerySuggestions();
	syncURL();
	void loadExplorer();
}

function scheduleSearch(value: string): void {
	state.searchText = value;
	syncURL();
	if (searchTimer !== null) window.clearTimeout(searchTimer);
	searchTimer = window.setTimeout(() => {
		state.pageToken = "";
		syncURL();
		void loadExplorer();
	}, 300);
}

function resetFilters(): void {
	state.query = state.draftQuery = "";
	state.searchText = "";
	state.container = "";
	state.stream = "";
	state.severity = "";
	state.duration = "5m";
	state.timeFrom = "";
	state.timeTo = "";
	state.showQuery = false;
	state.pageToken = "";
	if (searchTimer !== null) window.clearTimeout(searchTimer);
	clearError();
	syncURL();
	renderAll();
	void loadExplorer();
}

function setOneHourRange(): void {
	state.duration = "1h";
	state.timeFrom = "";
	state.timeTo = "";
	state.pageToken = "";
	syncURL();
	renderAll();
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
	const to = fromDateTimeLocal(($("#customToInput") as HTMLInputElement).value);
	if (!from || !to || new Date(from).getTime() >= new Date(to).getTime()) {
		showError(t("errors.customRange"));
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
	$$<HTMLElement>(".entry-row").forEach((row) => {
		row.classList.toggle(
			"selected",
			row.getAttribute("data-entry-id") === entryId,
		);
	});
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
	$$<HTMLElement>(".entry-row").forEach((row) => {
		row.classList.remove("selected");
	});
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
		onFieldFilter: (field, value) => {
			const clause = `${field} = "${value.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
			state.query = state.query.trim()
				? `${state.query.trim()} AND ${clause}`
				: clause;
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

function setupLocale(): void {
	$$<HTMLButtonElement>("[data-locale]").forEach((button) => {
		button.addEventListener("click", () => {
			const value = button.dataset.locale;
			if (!isSupportedLocale(value)) return;
			setLocale(value);
			state.tailMessage = "";
			$$<HTMLButtonElement>("[data-locale]").forEach((option) => {
				option.classList.toggle("selected", option === button);
				if (option === button) option.setAttribute("aria-current", "true");
				else option.removeAttribute("aria-current");
			});
			renderAll();
			renderErrorBanner();
			applyTheme(state.theme);
			closeHeaderMenus(true);
		});
	});
	const current = getLocale();
	$$<HTMLButtonElement>("[data-locale]").forEach((button) => {
		const selected = button.dataset.locale === current;
		button.classList.toggle("selected", selected);
		if (selected) button.setAttribute("aria-current", "true");
	});
}

function moveEntryFocus(delta: number, edge?: "first" | "last"): void {
	const rows = $$<HTMLButtonElement>(".entry-row");
	if (!rows.length) return;
	const activeIndex = rows.indexOf(document.activeElement as HTMLButtonElement);
	const selectedIndex = state.selectedId
		? rows.findIndex((row) => row.dataset.entryId === state.selectedId)
		: -1;
	const current = activeIndex >= 0 ? activeIndex : selectedIndex;
	const targetIndex =
		edge === "first"
			? 0
			: edge === "last"
				? rows.length - 1
				: Math.min(
						rows.length - 1,
						Math.max(0, (current < 0 ? 0 : current) + delta),
					);
	rows[targetIndex].focus();
}

function setupEvents(): void {
	$("#consoleMenuButton").addEventListener("click", toggleMobileNav);
	$("#mobileNavBackdrop").addEventListener("click", closeMobileOverlay);
	$("#headerMenuButton").addEventListener("click", () =>
		toggleMenu($("#headerMenu"), $("#headerMenuButton")),
	);
	$("#themeToggleButton").addEventListener("click", () => {
		applyTheme(state.theme === "dark" ? "light" : "dark");
		closeHeaderMenus(true);
	});
	$("#refreshButton").addEventListener("click", () => {
		state.pageToken = "";
		syncURL();
		closeHeaderMenus(true);
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
	$("#clearQueryButton").addEventListener("click", resetFilters);
	$("#searchAllFields").addEventListener("input", (event: Event) => {
		scheduleSearch((event.target as HTMLInputElement).value);
	});
	$("#showQueryButton").addEventListener("click", () => {
		state.showQuery = !state.showQuery;
		if (!state.showQuery) closeQuerySuggestions();
		syncURL();
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
		if (!target.closest(".query-help")) {
			$("#queryHelpPopover").setAttribute("hidden", "");
			$("#queryHelpButton").setAttribute("aria-expanded", "false");
		}
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
		syncURL();
		renderEntries();
	});
	$("#nextPageButton").addEventListener("click", () => {
		if (state.response?.nextPageToken) {
			state.pageToken = state.response.nextPageToken;
			syncURL();
			void loadExplorer(true);
		}
	});
	$("#applyCustomRangeButton").addEventListener("click", applyCustomRange);
	$("#clearCustomRangeButton").addEventListener("click", clearCustomRange);
	$("#closeDetailButton").addEventListener("click", closeDetail);
	$("#errorDismiss").addEventListener("click", () => clearError());
	$("#errorDetailsToggle").addEventListener("click", () => {
		const button = $("#errorDetailsToggle");
		const details = $("#errorDetails");
		const open = details.hasAttribute("hidden");
		details.toggleAttribute("hidden", !open);
		button.setAttribute("aria-expanded", String(open));
	});
	$("#entryList").addEventListener("click", (event: MouseEvent) => {
		const target = event.target as Element;
		const row = target.closest<HTMLButtonElement>(".entry-row");
		const entryId = row?.dataset.entryId;
		if (entryId) {
			openDetail(entryId);
			return;
		}
		const action = target.closest<HTMLElement>("[data-empty-action]")?.dataset
			.emptyAction;
		if (action === "reset") resetFilters();
		if (action === "hour") setOneHourRange();
	});
	$("#shareButton").addEventListener("click", () => {
		void copyText(window.location.href).then((copied) =>
			toast(copied ? t("common.linkCopied") : t("detail.copyFailed")),
		);
	});
	$("#queryHelpButton").addEventListener("click", () => {
		const button = $("#queryHelpButton");
		const popover = $("#queryHelpPopover");
		const open = popover.hasAttribute("hidden");
		popover.toggleAttribute("hidden", !open);
		button.setAttribute("aria-expanded", String(open));
	});
	$("#fieldsToggle").addEventListener("click", () => {
		state.fieldsHidden = !state.fieldsHidden;
		if (isMobileViewport() && state.fieldsHidden) {
			closeMobileOverlay();
			return;
		}
		if (isMobileViewport() && !state.fieldsHidden) {
			fieldsReturnFocus = $("#consoleMenuButton") as HTMLElement;
			syncMobileFieldsOverlay();
		}
		syncURL();
		renderAll();
	});
	$("#timelineToggle").addEventListener("click", () => {
		state.timelineHidden = !state.timelineHidden;
		syncURL();
		renderAll();
	});
	$("#timelineZoomOut").addEventListener("click", () => shiftTimelineRange(1));
	$("#timelineZoomIn").addEventListener("click", () => shiftTimelineRange(-1));
	window.addEventListener("popstate", () => {
		hydrateURL();
		syncMobileFieldsOverlay();
		void loadExplorer();
		setLive(state.live);
	});
	document.addEventListener("keydown", (event: KeyboardEvent) => {
		const activeTag = document.activeElement?.tagName;
		if (state.navExpanded && event.key === "Tab") {
			const focusable = $$<HTMLElement>("#sideNav button").filter(
				(element) => !element.hasAttribute("disabled"),
			);
			if (focusable.length) {
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
		}
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
			else if (
				state.navExpanded ||
				document.body.classList.contains("fields-open")
			)
				closeMobileOverlay();
			else {
				closeHeaderMenus(true);
				$("#queryHelpPopover").setAttribute("hidden", "");
				$("#queryHelpButton").setAttribute("aria-expanded", "false");
			}
		}
		const inTextControl =
			activeTag === "INPUT" ||
			activeTag === "TEXTAREA" ||
			activeTag === "SELECT" ||
			(document.activeElement as HTMLElement)?.isContentEditable;
		const activeRow = document.activeElement?.classList.contains("entry-row");
		if (!inTextControl && !$("#detailDrawer").hasAttribute("hidden")) return;
		if (
			!inTextControl &&
			(activeRow || event.key === "j" || event.key === "k")
		) {
			if (event.key === "ArrowDown" || event.key === "j") {
				event.preventDefault();
				moveEntryFocus(1);
			}
			if (event.key === "ArrowUp" || event.key === "k") {
				event.preventDefault();
				moveEntryFocus(-1);
			}
			if (event.key === "Home") {
				event.preventDefault();
				moveEntryFocus(0, "first");
			}
			if (event.key === "End") {
				event.preventDefault();
				moveEntryFocus(0, "last");
			}
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
if (
	!new URL(window.location.href).searchParams.has("fields") &&
	window.innerWidth > 1440
)
	state.fieldsHidden = false;
syncURL();
setupRenderActions();
setupEvents();
setLocale(getLocale());
setupLocale();
applyTheme(loadSavedTheme());
setupTimelineResolution(() => {
	if (!state.response) return;
	state.pageToken = "";
	syncURL();
	void loadExplorer();
});
syncMobileFieldsOverlay();
setLive(state.live);
void loadStatus();
void loadExplorer();
