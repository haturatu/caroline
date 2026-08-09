export const supportedLocales = ["en", "ja", "zh-CN", "zh-TW", "ru"] as const;

export type Locale = (typeof supportedLocales)[number];

const localeStorageKey = "caroline-locale";

export function isSupportedLocale(
	value: string | null | undefined,
): value is Locale {
	return (
		value != null && (supportedLocales as readonly string[]).includes(value)
	);
}

export function matchLocale(input: string | null | undefined): Locale | null {
	if (!input) return null;
	const locale = input.toLowerCase();
	if (locale.startsWith("ja")) return "ja";
	if (locale.startsWith("ru")) return "ru";
	if (locale === "zh-tw" || locale === "zh-hk" || locale.includes("hant"))
		return "zh-TW";
	if (locale.startsWith("zh")) return "zh-CN";
	if (locale.startsWith("en")) return "en";
	return null;
}

export function detectLocale(): Locale {
	try {
		const saved = window.localStorage.getItem(localeStorageKey);
		if (isSupportedLocale(saved)) return saved;
	} catch {
		// Local storage can be unavailable in private browsing contexts.
	}
	for (const locale of navigator.languages || []) {
		const matched = matchLocale(locale);
		if (matched) return matched;
	}
	return matchLocale(navigator.language) || "en";
}

export function saveLocale(locale: Locale): void {
	try {
		window.localStorage.setItem(localeStorageKey, locale);
	} catch {
		// Local storage can be unavailable in private browsing contexts.
	}
}
