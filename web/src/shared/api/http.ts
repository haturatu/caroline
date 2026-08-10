import { t } from "../i18n/index";

export async function getJSON<T>(url: string): Promise<T> {
	const response = await fetch(url, {
		headers: { Accept: "application/json" },
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
