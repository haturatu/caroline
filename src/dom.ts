export function $<T extends Element = HTMLElement>(selector: string): T {
	const element = document.querySelector<T>(selector);
	if (!element) throw new Error(`Missing UI element: ${selector}`);
	return element;
}

export function $$<T extends Element = HTMLElement>(selector: string): T[] {
	return [...document.querySelectorAll<T>(selector)];
}

const htmlEscapeMap: Record<string, string> = {
	"&": "&amp;",
	"<": "&lt;",
	">": "&gt;",
	"'": "&#039;",
	'"': "&quot;",
};

/** Escape API-provided values before interpolating them into HTML strings. */
export function escapeHTML(value: unknown): string {
	return String(value ?? "").replace(/[&<>'"]/g, (character) => htmlEscapeMap[character]);
}
