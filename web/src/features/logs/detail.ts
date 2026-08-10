import { $, escapeHTML } from "../../shared/dom/selectors";
import { formatTime } from "../../shared/format";
import { copyText } from "../../shared/dom/clipboard";
import { state } from "../../app/state";
import { t } from "../../shared/i18n/index";
import { getRenderActions } from "../explorer/actions";
import { entrySummary } from "./render";

export function renderDetail(): void {
	const entry = state.entries.find(
		(item) => item.insertId === state.selectedId,
	);
	const drawer = $("#detailDrawer") as HTMLDialogElement;
	if (!entry) {
		if (drawer.open) drawer.close();
		return;
	}
	const payload = entry.jsonPayload || {
		textPayload: entry.textPayload || entry.summary,
	};
	$("#detailTitle").textContent =
		`${entry.severity} · ${entry.resource.labels.container_name || t("common.containerLog")}`;
	$("#detailBody").innerHTML =
		`<section class="detail-section"><span class="detail-label">${t("detail.timestamp")}</span><div class="detail-value"><time datetime="${escapeHTML(entry.timestamp)}">${escapeHTML(formatTime(entry.timestamp))}</time></div></section><section class="detail-section"><span class="detail-label">${t("detail.summary")}</span><div class="detail-value">${escapeHTML(entrySummary(entry))}</div></section><section class="detail-section"><span class="detail-label">${t("detail.payload")}</span><pre class="detail-code">${escapeHTML(JSON.stringify(payload, null, 2))}</pre></section><section class="detail-section"><span class="detail-label">${t("detail.metadata")}</span><div class="detail-meta"><div class="detail-meta-row"><span>${t("detail.insertId")}</span><strong>${escapeHTML(entry.insertId)}</strong></div><div class="detail-meta-row"><span>${t("detail.logName")}</span><strong>${escapeHTML(entry.logName)}</strong></div><div class="detail-meta-row"><span>${t("detail.resourceType")}</span><strong>${escapeHTML(entry.resource.type)}</strong></div><div class="detail-meta-row"><span>${t("detail.stream")}</span><strong>${escapeHTML(entry.stream)}</strong></div></div></section><button class="run-button" id="copyEntryButton" type="button">${t("detail.copyEntry")}</button>`;
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
