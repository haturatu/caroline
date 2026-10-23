export function zonedInput(value: string, zone: string): string {
	const date = new Date(value);
	if (!Number.isFinite(date.getTime())) return "";
	if (zone === "UTC") return date.toISOString().slice(0, 19);
	const parts = new Intl.DateTimeFormat("sv-SE", {
		...(zone === "local" ? {} : { timeZone: zone }),
		year: "numeric",
		month: "2-digit",
		day: "2-digit",
		hour: "2-digit",
		minute: "2-digit",
		second: "2-digit",
		hourCycle: "h23",
	}).formatToParts(date);
	const part = (type: string) =>
		parts.find((item) => item.type === type)?.value;
	return `${part("year")}-${part("month")}-${part("day")}T${part("hour")}:${part("minute")}:${part("second")}`;
}
export function parseZonedInput(value: string, zone: string): string {
	if (!value) return "";
	const normalized = value.length === 16 ? `${value}:00` : value;
	if (zone === "local") {
		const date = new Date(normalized);
		return Number.isFinite(date.getTime()) &&
			zonedInput(date.toISOString(), zone) === normalized
			? date.toISOString()
			: "";
	}
	const target = Date.parse(`${normalized}Z`);
	if (!Number.isFinite(target)) return "";
	let candidate = target;
	for (let index = 0; index < 4; index++) {
		const current = zonedInput(new Date(candidate).toISOString(), zone);
		if (current === normalized) return new Date(candidate).toISOString();
		candidate += target - Date.parse(`${current}Z`);
	}
	return "";
}
