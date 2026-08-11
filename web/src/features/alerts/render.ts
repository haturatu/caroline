import { state } from "../../app/state";
import { escapeHTML, $ } from "../../shared/dom/selectors";
import { formatNumber, formatTime } from "../../shared/format";
import { t } from "../../shared/i18n/index";
import type { AlertRule } from "../../shared/types";

function statusKey(rule: AlertRule): "alerts.firing" | "alerts.ok" | "alerts.paused" {
	if (!rule.enabled) return "alerts.paused";
	return rule.status === "FIRING" ? "alerts.firing" : "alerts.ok";
}

function statusClass(rule: AlertRule): string {
	if (!rule.enabled) return "paused";
	return rule.status === "FIRING" ? "firing" : "ok";
}

function ruleMatchesFilter(rule: AlertRule, search: string, filter: string): boolean {
	const haystack = [
		rule.name,
		rule.query,
		rule.severity || "",
		...Object.entries(rule.labels || {}).flat(),
	]
		.join(" ")
		.toLocaleLowerCase();
	if (search && !haystack.includes(search.toLocaleLowerCase())) return false;
	if (filter === "PAUSED") return !rule.enabled;
	if (filter === "FIRING") return rule.enabled && rule.status === "FIRING";
	if (filter === "OK") return rule.enabled && rule.status !== "FIRING";
	return true;
}

function renderSummary(rules: AlertRule[]): void {
	const firing = rules.filter((rule) => rule.enabled && rule.status === "FIRING").length;
	const enabled = rules.filter((rule) => rule.enabled).length;
	const notifications = rules.filter((rule) => rule.webhookConfigured).length;
	$("#alertSummaryTotal").textContent = formatNumber(rules.length);
	$("#alertSummaryFiring").textContent = formatNumber(firing);
	$("#alertSummaryEnabled").textContent = formatNumber(enabled);
	$("#alertSummaryNotifications").textContent = formatNumber(notifications);
	$("#alertSummaryTotalDescription").textContent = t("alerts.summaryAll");
	$("#alertSummaryFiringDescription").textContent = t("alerts.summaryFiringDescription");
	$("#alertSummaryEnabledDescription").textContent = t("alerts.summaryEnabledDescription");
	$("#alertSummaryNotificationsDescription").textContent = t(
		"alerts.summaryNotificationsDescription",
	);
}

export function renderAlerts(rules: AlertRule[] = state.alerts.rules): void {
	renderSummary(rules);
	const searchInput = $("#alertSearchInput") as HTMLInputElement;
	const filterInput = $("#alertStatusFilter") as HTMLSelectElement;
	const search = searchInput.value.trim();
	const filter = filterInput.value || "all";
	state.alerts.search = search;
	state.alerts.statusFilter = filter as typeof state.alerts.statusFilter;
	const filtered = rules.filter((rule) => ruleMatchesFilter(rule, search, filter));
	$("#alertListCount").textContent = t("alerts.showing", {
		count: formatNumber(filtered.length),
		total: formatNumber(rules.length),
	});
	const list = $("#alertList");
	if (!rules.length) {
		list.innerHTML = `<p class="alerts-empty"><strong>${escapeHTML(t("alerts.emptyTitle"))}</strong>${escapeHTML(t("alerts.empty"))}</p>`;
		return;
	}
	if (!filtered.length) {
		list.innerHTML = `<p class="alerts-empty"><strong>${escapeHTML(t("alerts.noMatchTitle"))}</strong>${escapeHTML(t("alerts.noMatch"))}</p>`;
		return;
	}
	list.innerHTML = filtered
		.map((rule) => {
			const lastActivity = rule.firingSince || rule.lastFiredAt;
			const activity = lastActivity
				? formatTime(lastActivity)
				: t("alerts.neverFired");
			const notificationClass = rule.webhookConfigured ? "" : " no-webhook";
			const notificationLabel = rule.webhookConfigured
				? t("alerts.webhookConfigured")
				: t("alerts.noWebhook");
			return `<article class="alert-policy-card" data-alert-id="${escapeHTML(rule.id)}">
				<div class="alert-policy-main">
					<div class="alert-policy-title-row">
						<h3>${escapeHTML(rule.name)}</h3>
						<span class="alert-status alert-status-${statusClass(rule)}">${escapeHTML(t(statusKey(rule)))}</span>
					</div>
					<code class="alert-policy-query">${escapeHTML(rule.query || t("query.allLogs"))}</code>
					<div class="alert-policy-facts">
						<span class="alert-policy-fact">${escapeHTML(t("alerts.threshold", { count: formatNumber(rule.threshold) }))}</span>
						<span class="alert-policy-fact">${escapeHTML(t("alerts.windowValue", { seconds: formatNumber(rule.windowSeconds) }))}</span>
						<span class="alert-policy-fact">${escapeHTML(rule.severity ? `${t("alerts.severity")}: ${rule.severity}` : t("alerts.severityNone"))}</span>
					</div>
				</div>
				<div class="alert-policy-meta">
					<div class="alert-policy-notification${notificationClass}">
						<span class="notification-dot" aria-hidden="true"></span>
						<span>${escapeHTML(notificationLabel)}</span>
					</div>
					<span>${escapeHTML(t("alerts.lastActivity", { time: activity }))}</span>
					<div class="alert-policy-actions">
						<button class="text-button" type="button" data-alert-edit="${escapeHTML(rule.id)}">${escapeHTML(t("alerts.edit"))}</button>
						<button class="text-button" type="button" data-alert-toggle="${escapeHTML(rule.id)}" aria-pressed="${String(!rule.enabled)}">${escapeHTML(rule.enabled ? t("alerts.pause") : t("alerts.resume"))}</button>
						<button class="text-button danger-button" type="button" data-alert-delete="${escapeHTML(rule.id)}">${escapeHTML(t("alerts.delete"))}</button>
					</div>
				</div>
			</article>`;
		})
		.join("");
}

export function renderAlertLoading(): void {
	$("#alertSummaryTotal").textContent = "—";
	$("#alertSummaryFiring").textContent = "—";
	$("#alertSummaryEnabled").textContent = "—";
	$("#alertSummaryNotifications").textContent = "—";
	$("#alertList").innerHTML =
		`<p class="alerts-empty">${escapeHTML(t("alerts.loading"))}</p>`;
}

export function renderAlertError(): void {
	$("#alertList").innerHTML =
		`<p class="alerts-empty"><strong>${escapeHTML(t("alerts.loadFailed"))}</strong>${escapeHTML(state.alerts.error)}</p>`;
}
