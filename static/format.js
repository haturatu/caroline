export function formatNumber(value) {
    return new Intl.NumberFormat(undefined).format(value || 0);
}
export function formatTime(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime())
        ? "—"
        : new Intl.DateTimeFormat(undefined, {
            dateStyle: "short",
            timeStyle: "medium",
        }).format(date);
}
export function formatTimelineTick(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime())
        ? "—"
        : new Intl.DateTimeFormat(undefined, {
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
            hour12: false,
        }).format(date);
}
export function severityClass(value) {
    return value.toLowerCase() === "warning"
        ? "warning"
        : value.toLowerCase() === "error" ||
            value.toLowerCase() === "critical" ||
            value.toLowerCase() === "alert" ||
            value.toLowerCase() === "emergency"
            ? "error"
            : value.toLowerCase() === "debug"
                ? "debug"
                : "info";
}
export function errorText(error) {
    return error instanceof Error
        ? error.message
        : "Something went wrong—try again.";
}
