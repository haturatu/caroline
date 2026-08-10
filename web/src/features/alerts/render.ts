import { state } from "../../app/state";
import { escapeHTML, $ } from "../../shared/dom/selectors";
import { formatNumber, formatTime } from "../../shared/format";
import { t, tp } from "../../shared/i18n/index";
import type { AlertRule } from "../../shared/types";

export function renderAlerts(rules: AlertRule[] = state.alerts.rules): void {
	const list = $("#alertList");
	if (!rules.length) {
		list.innerHTML = `<p class="alerts-empty">${escapeHTML(t("alerts.empty"))}</p>`;
		return;
	}
	list.innerHTML = rules
		.map((rule) => {
			const statusKey =
				rule.status === "FIRING" ? "alerts.firing" : "alerts.ok";
			const lastFired = rule.lastFiredAt
				? formatTime(rule.lastFiredAt)
				: t("alerts.neverFired");
			return `<article class="alert-card" data-alert-id="${escapeHTML(rule.id)}">
				<div class="alert-card-main">
					<div class="alert-card-title-row">
						<h3>${escapeHTML(rule.name)}</h3>
						<span class="alert-status alert-status-${rule.status.toLowerCase()}">${escapeHTML(t(statusKey))}</span>
					</div>
					<code class="alert-query">${escapeHTML(rule.query || t("query.allLogs"))}</code>
					<p class="alert-card-meta">${escapeHTML(tp("alerts.matches", rule.matchCount, { count: formatNumber(rule.matchCount) }))} · ${escapeHTML(t("alerts.threshold", { count: formatNumber(rule.threshold) }))} · ${escapeHTML(t("alerts.lastFired", { time: lastFired }))}</p>
				</div>
				<button class="text-button alert-delete-button" type="button" data-alert-delete="${escapeHTML(rule.id)}">${escapeHTML(t("alerts.delete"))}</button>
			</article>`;
		})
		.join("");
}

export function renderAlertLoading(): void {
	$("#alertList").innerHTML =
		`<p class="alerts-empty">${escapeHTML(t("alerts.loading"))}</p>`;
}

export function renderAlertError(): void {
	$("#alertList").innerHTML =
		`<p class="alerts-empty">${escapeHTML(t("alerts.loadFailed"))}</p>`;
}
