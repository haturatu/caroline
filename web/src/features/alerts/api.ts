import { getJSON, requestJSON } from "../../shared/api/http";
import type { AlertRule } from "../../shared/types";

export type { AlertRule };

export type AlertRuleInput = {
	name: string;
	query: string;
	severity: string;
	labels: Record<string, string>;
	runbookUrl: string;
	sampleMode: "off" | "summary" | "full";
	threshold: number;
	windowSeconds: number;
	cooldownSeconds: number;
	enabled: boolean;
	webhookUrl: string;
};

export function fetchAlerts(): Promise<AlertRule[]> {
	return getJSON<AlertRule[]>("/api/alerts");
}

export function createAlert(input: AlertRuleInput): Promise<AlertRule> {
	return requestJSON<AlertRule>("/api/alerts", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(input),
	});
}

export function deleteAlert(id: string): Promise<void> {
	return requestJSON<void>(`/api/alerts/${encodeURIComponent(id)}`, {
		method: "DELETE",
	});
}
