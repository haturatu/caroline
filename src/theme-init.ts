const themeStorageKey = "caroline-theme";
const localeStorageKey = "caroline-locale";

function matchLocale(input: string | null): string | null {
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

function applySavedLocale(): void {
	try {
		const saved = window.localStorage.getItem(localeStorageKey);
		const locale =
			(saved && matchLocale(saved) === saved ? saved : null) ||
			navigator.languages.map(matchLocale).find(Boolean) ||
			matchLocale(navigator.language) ||
			"en";
		document.documentElement.lang = locale;
	} catch {
		document.documentElement.lang = "en";
	}
}

function applySavedTheme(): void {
	try {
		if (window.localStorage.getItem(themeStorageKey) === "light")
			document.documentElement.dataset.theme = "light";
	} catch {
		// Local storage can be unavailable before the application initializes.
	}
}

applySavedTheme();
applySavedLocale();
