import { t } from "../i18n/index";

export async function requestJSON<T>(
	url: string,
	init: RequestInit = {},
): Promise<T> {
	const response = await fetch(url, {
		...init,
		headers: {
			Accept: "application/json",
			...init.headers,
		},
	});
	const payload = (await response.json().catch(() => ({}))) as {
		error?: string;
	};
	if (!response.ok) {
		if (payload.error?.startsWith("Docker daemon is unavailable"))
			throw new Error(t("status.connectionError"));
		throw new Error(
			payload.error || t("errors.requestFailed", { status: response.status }),
		);
	}
	return payload as T;
}

export function getJSON<T>(url: string): Promise<T> {
	return requestJSON<T>(url);
}
