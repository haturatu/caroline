const themeStorageKey = "caroline-theme";

function applySavedTheme(): void {
	try {
		if (window.localStorage.getItem(themeStorageKey) === "light")
			document.documentElement.dataset.theme = "light";
	} catch {
		// Local storage can be unavailable before the application initializes.
	}
}

applySavedTheme();
