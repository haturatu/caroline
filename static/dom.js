export function $(selector) {
    const element = document.querySelector(selector);
    if (!element)
        throw new Error(`Missing UI element: ${selector}`);
    return element;
}
export function $$(selector) {
    return [...document.querySelectorAll(selector)];
}
export function escapeHTML(value) {
    return String(value ?? "").replace(/[&<>'"]/g, (c) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "'": "&#039;",
        '"': "&quot;",
    })[c] || c);
}
