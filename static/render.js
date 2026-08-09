import { $$, $, escapeHTML } from "./dom.js";
import { formatNumber, formatTime, formatTimelineTick, severityClass, } from "./format.js";
import { state } from "./state.js";
let actions = {};
export function setRenderActions(nextActions) {
    actions = nextActions;
}
export function renderLoading() {
    $("#fieldGroups").innerHTML =
        '<div class="panel-loading loading-panel"><span class="spinner"></span><span>Loading fields…</span></div>';
    $("#timelineChart").innerHTML =
        '<div class="chart-loading"><span class="spinner"></span><span>Loading timeline…</span></div>';
    $("#timelineAxis").innerHTML = "";
    $("#entryList").innerHTML =
        '<div class="empty-state"><span class="spinner"></span><strong>Loading Logs…</strong><p>Reading the latest entries from Docker Engine.</p></div>';
}
export function renderFilters() {
    const select = $("#containerFilter");
    const options = [
        '<option value="">All Containers</option>',
        ...state.containers.map((container) => `<option value="${escapeHTML(container.id)}">${escapeHTML(container.name)}</option>`),
    ];
    select.innerHTML = options.join("");
    select.value = state.container;
    $("#streamFilter").value = state.stream;
    $("#severityFilter").value = state.severity;
    $("#rangeFilter").value = state.timeFrom
        ? "custom"
        : state.duration;
    const customRange = $("#customRangeEditor");
    customRange.toggleAttribute("hidden", !state.timeFrom || !state.timeTo);
    $("#customFromInput").value = state.timeFrom
        ? toDateTimeLocal(state.timeFrom)
        : "";
    $("#customToInput").value = state.timeTo
        ? toDateTimeLocal(state.timeTo)
        : "";
    $("#sortFilter").value = state.sort;
    $("#queryInput").value = state.draftQuery;
    $("#searchAllFields").value = state.searchText;
    $("#queryEditor").toggleAttribute("hidden", !state.showQuery);
    $("#showQueryButton").textContent = state.showQuery
        ? "Hide query"
        : "Show query";
}
function toDateTimeLocal(value) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime()))
        return "";
    const pad = (part) => String(part).padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
export function renderFields() {
    const target = $("#fieldGroups");
    if (state.loading && !state.response) {
        target.innerHTML =
            '<div class="panel-loading loading-panel"><span class="spinner"></span><span>Loading fields…</span></div>';
        return;
    }
    if (!state.response?.fields.length) {
        target.innerHTML =
            '<div class="panel-loading">No fields were found in the current result set.</div>';
        return;
    }
    target.innerHTML = state.response.fields
        .map((group) => `<section class="field-group"><div class="field-group-title">${escapeHTML(group.name)}</div>${group.fields
        .map((field) => `<button class="field-row" type="button" aria-expanded="false" data-field="${escapeHTML(field.name)}"><span class="field-name" title="${escapeHTML(field.name)}">${escapeHTML(field.name)}</span><span class="field-count">${formatNumber(field.count)}</span></button><div class="field-values">${Object.entries(field.values || {})
        .slice(0, 5)
        .map(([value, count]) => `<button class="field-value" type="button" data-field="${escapeHTML(field.name)}" data-value="${escapeHTML(value)}">${escapeHTML(value)} <span>(${formatNumber(count)})</span></button>`)
        .join("")}</div>`)
        .join("")}</section>`)
        .join("");
    $$(".field-row").forEach((button) => button.addEventListener("click", () => {
        const expanded = !button.classList.contains("expanded");
        button.classList.toggle("expanded", expanded);
        button.setAttribute("aria-expanded", String(expanded));
    }));
    $$(".field-value").forEach((button) => button.addEventListener("click", () => {
        const field = button.getAttribute("data-field");
        const value = button.getAttribute("data-value");
        if (field && value)
            actions.onFieldFilter?.(field, value);
    }));
}
function timelineSegment(bucket, name, className) {
    return `<span class="timeline-segment ${className}" style="height:${Math.max(0, ((bucket.severities[name] || 0) / Math.max(1, bucket.total)) * 100)}%"></span>`;
}
function timelineBucketLabel(bucket) {
    const start = Date.parse(bucket.start);
    const end = Date.parse(bucket.end);
    const midpoint = Number.isNaN(start) || Number.isNaN(end)
        ? bucket.start
        : new Date((start + end) / 2).toISOString();
    return `${formatTimelineTick(midpoint)} · ${formatNumber(bucket.total)} entries`;
}
function timelineSelection(response) {
    const from = Date.parse(response.from);
    const to = Date.parse(response.to);
    const selectedFrom = state.timeFrom ? Date.parse(state.timeFrom) : from;
    const selectedTo = state.timeTo ? Date.parse(state.timeTo) : to;
    if (Number.isNaN(from) ||
        Number.isNaN(to) ||
        to <= from ||
        Number.isNaN(selectedFrom) ||
        Number.isNaN(selectedTo))
        return { left: 0, right: 0 };
    const clamp = (value) => Math.min(100, Math.max(0, value));
    const left = clamp(((selectedFrom - from) / (to - from)) * 100);
    const end = clamp(((selectedTo - from) / (to - from)) * 100);
    return { left, right: 100 - Math.max(left, end) };
}
export function renderTimeline() {
    const response = state.response;
    const chart = $("#timelineChart");
    const axis = $("#timelineAxis");
    if (state.loading && !response) {
        chart.innerHTML =
            '<div class="chart-loading"><span class="spinner"></span><span>Loading timeline…</span></div>';
        axis.innerHTML = "";
        return;
    }
    if (!response?.timeline.length) {
        chart.innerHTML =
            '<div class="chart-loading">No timeline data for this query.</div>';
        axis.innerHTML = "";
        return;
    }
    const maximum = Math.max(1, ...response.timeline.map((bucket) => bucket.total));
    const bars = response.timeline
        .map((bucket, index) => {
        if (!bucket.total)
            return "";
        const height = Math.max(4, Math.round((bucket.total / maximum) * 68));
        const position = ((index + 0.5) / response.timeline.length) * 100;
        const rangeLabel = `${formatTime(bucket.start)} to ${formatTime(bucket.end)}`;
        return `<button class="timeline-bar" type="button" data-start="${escapeHTML(bucket.start)}" data-end="${escapeHTML(bucket.end)}" style="--bar-position:${position}%;--bar-height:${height}px" aria-label="Set time range to ${escapeHTML(rangeLabel)}; ${bucket.total} entries"><span class="timeline-bar-inner">${timelineSegment(bucket, "ERROR", "error")}${timelineSegment(bucket, "WARNING", "warning")}${timelineSegment(bucket, "INFO", "info")}${timelineSegment(bucket, "DEBUG", "debug")}</span></button>`;
    })
        .join("");
    const selection = timelineSelection(response);
    const initialBucket = response.timeline[Math.floor(response.timeline.length / 2)] ||
        response.timeline[0];
    chart.innerHTML = `<div class="timeline-grid" aria-hidden="true"><span></span><span></span><span></span><span></span></div><div class="timeline-selection" style="left:${selection.left}%;right:${selection.right}%" aria-hidden="true"><span class="timeline-handle timeline-handle-start"></span><span class="timeline-handle timeline-handle-end"></span></div><div class="timeline-bars">${bars}</div><div class="timeline-baseline" aria-hidden="true"></div><div class="timeline-cursor" style="left:50%" aria-hidden="true"><span class="timeline-cursor-badge" title="${escapeHTML(timelineBucketLabel(initialBucket))}">${escapeHTML(timelineBucketLabel(initialBucket))}</span></div>`;
    const tickCount = 7;
    axis.innerHTML = Array.from({ length: tickCount }, (_, index) => {
        const bucketIndex = Math.min(response.timeline.length - 1, Math.round((index * (response.timeline.length - 1)) / (tickCount - 1)));
        const value = index === 0
            ? response.from
            : index === tickCount - 1
                ? response.to
                : response.timeline[bucketIndex].start;
        return `<span>${formatTimelineTick(value)}</span>`;
    }).join("");
    if (!chart.dataset.cursorBound) {
        chart.addEventListener("mousemove", (event) => {
            const bounds = chart.getBoundingClientRect();
            const position = Math.min(1, Math.max(0, (event.clientX - bounds.left) / bounds.width));
            const cursor = chart.querySelector(".timeline-cursor");
            const timeline = state.response?.timeline || [];
            if (cursor)
                cursor.style.left = `${position * 100}%`;
            if (timeline.length) {
                const bucket = timeline[Math.min(timeline.length - 1, Math.floor(position * timeline.length))];
                const label = timelineBucketLabel(bucket);
                const badge = chart.querySelector(".timeline-cursor-badge");
                if (badge) {
                    badge.textContent = label;
                    badge.title = label;
                }
            }
        });
        chart.addEventListener("mouseleave", () => {
            const cursor = chart.querySelector(".timeline-cursor");
            const timeline = state.response?.timeline || [];
            if (cursor)
                cursor.style.left = "50%";
            if (timeline.length) {
                const badge = chart.querySelector(".timeline-cursor-badge");
                if (badge) {
                    const label = timelineBucketLabel(timeline[Math.floor(timeline.length / 2)]);
                    badge.textContent = label;
                    badge.title = label;
                }
            }
        });
        chart.dataset.cursorBound = "true";
    }
    $$(".timeline-bar").forEach((bar) => bar.addEventListener("click", () => {
        const start = bar.getAttribute("data-start");
        const end = bar.getAttribute("data-end");
        if (start && end)
            actions.onTimelineSelect?.(start, end);
    }));
}
export function entrySummary(entry) {
    return (entry.summary ||
        entry.textPayload ||
        (entry.jsonPayload ? JSON.stringify(entry.jsonPayload) : ""));
}
export function renderEntries() {
    const list = $("#entryList");
    if (state.loading && !state.response) {
        list.innerHTML =
            '<div class="empty-state"><span class="spinner"></span><strong>Loading Logs…</strong><p>Reading the latest entries from Docker Engine.</p></div>';
        return;
    }
    if (!state.entries.length) {
        list.innerHTML = `<div class="empty-state"><span class="empty-state-icon" aria-hidden="true">⌕</span><strong>${state.response ? "No Matching Log Entries" : "Run a Query"}</strong><p>${state.response ? "Try a broader time range or a less specific query." : "Your matching log entries will appear here."}</p></div>`;
        return;
    }
    list.classList.toggle("wrap-lines", state.wrap);
    list.innerHTML = state.entries
        .map((entry) => `<button class="entry-row ${state.selectedId === entry.insertId ? "selected" : ""}" type="button" data-entry-id="${escapeHTML(entry.insertId)}" aria-label="${escapeHTML(`${entry.severity} log from ${entry.resource.labels.container_name}: ${entrySummary(entry)}`)}"><span class="entry-time">${escapeHTML(formatTime(entry.timestamp))}</span><span class="entry-severity ${severityClass(entry.severity)}">${escapeHTML(entry.severity)}</span><span class="entry-resource"><span class="resource-name">${escapeHTML(entry.resource.labels.container_name || "Unknown Container")}</span><span class="resource-meta">${escapeHTML(entry.stream)} · ${escapeHTML(entry.resource.type)}</span></span><span class="entry-summary" title="${escapeHTML(entrySummary(entry))}">${escapeHTML(entrySummary(entry))}</span><span class="entry-open"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 5 7 7-7 7"/></svg></span></button>`)
        .join("");
    $$(".entry-row").forEach((row) => row.addEventListener("click", () => {
        const entryId = row.getAttribute("data-entry-id");
        if (entryId)
            actions.onEntrySelect?.(entryId);
    }));
}
export function renderDetail() {
    const entry = state.entries.find((item) => item.insertId === state.selectedId);
    const drawer = $("#detailDrawer");
    if (!entry) {
        drawer.setAttribute("hidden", "");
        return;
    }
    drawer.removeAttribute("hidden");
    const payload = entry.jsonPayload || {
        textPayload: entry.textPayload || entry.summary,
    };
    $("#detailTitle").textContent =
        `${entry.severity} · ${entry.resource.labels.container_name || "Container Log"}`;
    $("#detailBody").innerHTML =
        `<section class="detail-section"><span class="detail-label">Timestamp</span><div class="detail-value">${escapeHTML(formatTime(entry.timestamp))}</div></section><section class="detail-section"><span class="detail-label">Summary</span><div class="detail-value">${escapeHTML(entrySummary(entry))}</div></section><section class="detail-section"><span class="detail-label">Payload</span><pre class="detail-code">${escapeHTML(JSON.stringify(payload, null, 2))}</pre></section><section class="detail-section"><span class="detail-label">Log Entry Metadata</span><div class="detail-meta"><div class="detail-meta-row"><span>Insert ID</span><strong>${escapeHTML(entry.insertId)}</strong></div><div class="detail-meta-row"><span>Log Name</span><strong>${escapeHTML(entry.logName)}</strong></div><div class="detail-meta-row"><span>Resource Type</span><strong>${escapeHTML(entry.resource.type)}</strong></div><div class="detail-meta-row"><span>Stream</span><strong>${escapeHTML(entry.stream)}</strong></div></div></section><button class="run-button" id="copyEntryButton" type="button">Copy Entry JSON</button>`;
    $("#copyEntryButton").addEventListener("click", () => {
        const copy = navigator.clipboard?.writeText(JSON.stringify(entry, null, 2));
        if (copy)
            void copy.then(() => actions.onToast?.("Entry JSON copied."));
    });
}
export function renderResultsMeta() {
    const response = state.response;
    $("#resultCount").textContent = response
        ? `${formatNumber(response.total)} Entries`
        : "—";
    $("#approximateBadge").toggleAttribute("hidden", !response?.approximate);
    $("#resultsDescription").textContent = response
        ? `${formatTime(response.from)} – ${formatTime(response.to)} · ${state.live ? "Live refresh enabled" : "Refresh paused"}`
        : "Run a query to see matching log entries.";
    $("#resultsFooter").textContent = state.live
        ? "Polling Docker Engine Every 5 Seconds."
        : "Live refresh is paused.";
    const next = $("#nextPageButton");
    next.toggleAttribute("hidden", !response?.nextPageToken);
    next.disabled = state.loading;
    next.textContent = state.loading
        ? "Loading…"
        : response?.nextPageToken
            ? "Load More"
            : "No More Results";
}
export function renderAll() {
    renderFilters();
    renderFields();
    renderTimeline();
    renderEntries();
    renderResultsMeta();
    $("#fields").classList.toggle("fields-collapsed", state.fieldsHidden);
    $("#timeline").classList.toggle("timeline-collapsed", state.timelineHidden);
    $("#wrapButton").setAttribute("aria-pressed", String(state.wrap));
    $("#streamButton").classList.toggle("paused", !state.live);
    $("#streamButton").setAttribute("aria-pressed", String(state.live));
    $("#streamButtonText").textContent = state.live
        ? "Auto-refresh On"
        : "Auto-refresh Off";
    $("#fieldsToggle").textContent = state.fieldsHidden ? "›" : "‹";
    $("#fieldsToggle").setAttribute("aria-label", state.fieldsHidden ? "Show Fields" : "Hide Fields");
    $("#fieldsToggle").setAttribute("title", state.fieldsHidden ? "Show Fields" : "Hide Fields");
}
