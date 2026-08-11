import { $, escapeHTML } from "../../shared/dom/selectors";
import { formatTime, severityClass } from "../../shared/format";
import { copyText } from "../../shared/dom/clipboard";
import { state } from "../../app/state";
import { t } from "../../shared/i18n/index";
import { getRenderActions } from "../explorer/actions";

export function renderDetail(): void {
	const entry = state.entries.find(
		(item) => item.insertId === state.selectedId,
	);
	const drawer = $("#detailDrawer") as HTMLDialogElement;
	const detailBody = $("#detailBody");
	const detailFooter = $("#detailFooter");
	if (!entry) {
		detailBody.innerHTML = "";
		detailFooter.innerHTML = "";
		detailFooter.setAttribute("hidden", "");
		if (drawer.open) drawer.close();
		return;
	}
	const labels = entry.resource.labels;
	const containerName = labels.container_name || t("common.containerLog");
	const nodeName = labels.node_name;
	const severity = entry.severity || t("common.notAvailable");
	const hasJSONPayload = Boolean(
		entry.jsonPayload && Object.keys(entry.jsonPayload).length > 0,
	);
	const textPayload = entry.textPayload || (!hasJSONPayload ? entry.summary : "");
	const unavailable = t("common.notAvailable");
	const metadataRow = (label: string, value: string | undefined) =>
		`<div class="detail-meta-row"><span>${label}</span><strong title="${escapeHTML(value || unavailable)}">${escapeHTML(value || unavailable)}</strong></div>`;

	detailBody.innerHTML = `
		<div class="detail-overview">
			<div class="detail-overview-heading">
				<span class="detail-severity ${severityClass(entry.severity)}">${escapeHTML(severity)}</span>
				<span class="detail-container-name" title="${escapeHTML(containerName)}">${escapeHTML(containerName)}</span>
			</div>
			<time class="detail-timestamp" datetime="${escapeHTML(entry.timestamp)}">${escapeHTML(formatTime(entry.timestamp))}</time>
		</div>
		${textPayload ? `<section class="detail-section"><span class="detail-label">${t("detail.payload")}</span><div class="detail-message">${escapeHTML(textPayload)}</div></section>` : ""}
		${hasJSONPayload ? `<section class="detail-section"><span class="detail-label">${t("detail.jsonPayload")}</span><pre class="detail-code">${escapeHTML(JSON.stringify(entry.jsonPayload, null, 2))}</pre></section>` : ""}
		<section class="detail-section detail-metadata-section"><span class="detail-label">${t("detail.metadata")}</span><div class="detail-meta">
			${metadataRow(t("detail.container"), containerName)}
			${nodeName ? metadataRow(t("detail.node"), nodeName) : ""}
			${metadataRow(t("detail.resourceType"), entry.resource.type)}
			${metadataRow(t("detail.stream"), entry.stream)}
			${metadataRow(t("detail.insertId"), entry.insertId)}
			${metadataRow(t("detail.logName"), entry.logName)}
		</div></section>`;
	detailFooter.removeAttribute("hidden");
	detailFooter.innerHTML = `<button class="run-button" id="copyEntryButton" type="button">${t("detail.copyEntry")}</button>`;
	$("#copyEntryButton").addEventListener("click", () => {
		const button = $("#copyEntryButton") as HTMLButtonElement;
		const originalLabel = t("detail.copyEntry");
		void copyText(JSON.stringify(entry, null, 2)).then((copied) => {
			if (!copied) {
				getRenderActions().onToast?.(t("detail.copyFailed"));
				return;
			}
			button.textContent = t("detail.copied");
			getRenderActions().onToast?.(t("detail.copied"));
			window.setTimeout(() => {
				if (button.isConnected) button.textContent = originalLabel;
			}, 1600);
		});
	});
}
