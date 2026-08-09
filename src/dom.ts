export function $<T extends Element = HTMLElement>(selector: string): T {
	const element = document.querySelector<T>(selector);
	if (!element) throw new Error(`Missing UI element: ${selector}`);
	return element;
}

export function $$<T extends Element = HTMLElement>(selector: string): T[] {
	return [...document.querySelectorAll<T>(selector)];
}

export function escapeHTML(value: unknown): string {
	return String(value ?? "").replace(
		/[&<>'"]/g,
		(c) =>
			({
				"&": "&amp;",
				"<": "&lt;",
				">": "&gt;",
				"'": "&#039;",
				'"': "&quot;",
			})[c] || c,
	);
}
