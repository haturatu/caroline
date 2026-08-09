export function formatNumber(value: number): string {
	return new Intl.NumberFormat(undefined).format(value || 0);
}

export function formatTime(value: string): string {
	const date = new Date(value);
	return Number.isNaN(date.getTime())
		? "—"
		: new Intl.DateTimeFormat(undefined, {
				dateStyle: "short",
				timeStyle: "medium",
			}).format(date);
}

export function formatClockTime(value: string): string {
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

export function formatTimelineTick(value: string): string {
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

export function formatTimelineAxisTick(
	value: string,
	from: string,
	to: string,
): string {
	const date = new Date(value);
	const start = Date.parse(from);
	const end = Date.parse(to);
	if (Number.isNaN(date.getTime()) || !Number.isFinite(start) || !Number.isFinite(end))
		return "—";

	const duration = Math.max(0, end - start);
	const showDate = duration >= 12 * 60 * 60 * 1000;
	const showWeekday = duration >= 3 * 24 * 60 * 60 * 1000;
	return new Intl.DateTimeFormat(undefined, {
		...(showDate ? { month: "2-digit", day: "2-digit" } : {}),
		...(showWeekday ? { weekday: "short" } : {}),
		hour: !showWeekday ? "2-digit" : undefined,
		minute: !showWeekday ? "2-digit" : undefined,
		hour12: false,
	}).format(date);
}

export function severityClass(value: string): string {
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

export function errorText(error: unknown): string {
	return error instanceof Error
		? error.message
		: "Something went wrong—try again.";
}
