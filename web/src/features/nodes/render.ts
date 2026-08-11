import { state } from "../../app/state";
import { $, escapeHTML } from "../../shared/dom/selectors";
import { formatTime } from "../../shared/format";
import { t } from "../../shared/i18n/index";
import type { NodeInfo } from "../../shared/types";

function statusLabel(status: string): string {
	return t(`nodes.status.${status}`) === `nodes.status.${status}`
		? status
		: t(`nodes.status.${status}`);
}

function statusClass(status: string): string {
	return status === "online"
		? "online"
		: status === "revoked"
			? "revoked"
			: "offline";
}

export function renderNodes(nodes: NodeInfo[] = state.nodes.items): void {
	const list = $("#nodeList");
	if (!nodes.length) {
		list.innerHTML = `<p class="nodes-empty">${escapeHTML(t("nodes.empty"))}</p>`;
		return;
	}
	list.innerHTML = nodes
		.map(
			(node) => `<article class="node-card">
				<div class="node-card-main">
					<div class="node-title-row">
						<h2>${escapeHTML(node.name || node.id)}</h2>
						<span class="node-status node-status-${statusClass(node.status)}">${escapeHTML(statusLabel(node.status))}</span>
					</div>
					<code>${escapeHTML(node.id)}</code>
					<div class="node-facts">
						<span>${escapeHTML(node.hostname || t("common.notAvailable"))}</span>
						<span>${escapeHTML([node.os, node.architecture].filter(Boolean).join(" / ") || t("common.notAvailable"))}</span>
						<span>${escapeHTML(t("nodes.lastSeen", { time: formatTime(node.lastSeenAt || "") }))}</span>
					</div>
				</div>
				<div class="node-card-actions">
					<button class="text-button danger-button" type="button" data-node-revoke="${escapeHTML(node.id)}"${node.status === "revoked" ? " disabled" : ""}>${escapeHTML(t("nodes.revoke"))}</button>
				</div>
			</article>`,
		)
		.join("");
}

export function renderNodesLoading(): void {
	$("#nodeList").innerHTML = `<p class="nodes-empty">${escapeHTML(t("nodes.loading"))}</p>`;
}

export function renderNodesError(message: string): void {
	$("#nodeList").innerHTML = `<p class="nodes-empty"><strong>${escapeHTML(t("nodes.loadFailed"))}</strong> ${escapeHTML(message)}</p>`;
}

export function renderEnrollmentToken(token: string, expiresAt: string): void {
	const panel = $("#nodeEnrollmentResult");
	panel.removeAttribute("hidden");
	$("#nodeEnrollmentToken").textContent = token;
	$("#nodeEnrollmentExpires").textContent = t("nodes.expires", { time: formatTime(expiresAt) });
}
