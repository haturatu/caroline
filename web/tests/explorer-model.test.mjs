import assert from "node:assert/strict";
import { test } from "node:test";
import {
	readQueries,
	exportEntries,
	snapshot,
	valueAt,
	shiftRange,
} from "../src/features/workspace/model.ts";
import { zonedInput, parseZonedInput } from "../src/features/workspace/time.ts";

const entry = {
	insertId: "one",
	timestamp: "2026-10-23T16:49:11.005Z",
	severity: "ERROR",
	stream: "stderr",
	logName: "containers/api/stderr",
	resource: { type: "docker_container", labels: { container_name: "api" } },
	labels: {},
	summary: "=1+1",
	textPayload: "=1+1",
	jsonPayload: {
		message: 'say "hello",\nnext',
		nested: { value: false },
		empty: "",
	},
};
const query = {
	query: "severity >= ERROR",
	searchText: "",
	logName: "",
	resourceType: "",
	node: "",
	container: "",
	stream: "",
	severity: "",
	duration: "5m",
	timeFrom: "",
	timeTo: "",
};
test("query snapshots persist filters without entries or secrets", () => {
	assert.deepEqual(
		snapshot({ ...query, entries: [entry], webhookURL: "secret" }),
		query,
	);
	const saved = {
		id: "a",
		name: "<script>",
		snapshot: query,
		updatedAt: new Date().toISOString(),
	};
	assert.deepEqual(
		readQueries({ getItem: () => JSON.stringify([null, {}, saved]) }, "test"),
		[saved],
	);
	assert.deepEqual(readQueries({ getItem: () => "{broken" }, "test"), []);
	assert.deepEqual(
		readQueries(
			{
				getItem: () => {
					throw Error("disabled");
				},
			},
			"test",
		),
		[],
	);
	assert.equal(
		readQueries(
			{ getItem: () => JSON.stringify(Array(101).fill(saved)) },
			"test",
		).length,
		100,
	);
});
test("summary field lookup handles missing and false values without inherited properties", () => {
	assert.equal(valueAt(entry, "jsonPayload.nested.value"), false);
	assert.equal(valueAt(entry, "jsonPayload.empty"), "");
	assert.equal(valueAt(entry, "jsonPayload.nope"), undefined);
	assert.equal(valueAt(entry, "constructor"), undefined);
});
test("JSON export preserves entries and CSV protects spreadsheet formulas and quoted newlines", () => {
	assert.deepEqual(JSON.parse(exportEntries([entry], "json")), [entry]);
	const csv = exportEntries([entry], "csv");
	assert.ok(csv.includes('"\'=1+1"'));
	assert.ok(csv.includes('"{""message"":""say \\""hello\\"",\\nnext""'));
	assert.ok(csv.startsWith('"timestamp","severity"'));
});
test("time navigation shifts by the selected interval", () => {
	assert.deepEqual(
		shiftRange("2026-10-23T16:00:00Z", "2026-10-23T16:05:00Z", -1),
		["2026-10-23T15:55:00.000Z", "2026-10-23T16:00:00.000Z"],
	);
});
test("custom ranges round trip in UTC and IANA zones, rejecting nonexistent DST times", () => {
	assert.equal(
		zonedInput(entry.timestamp, "Asia/Tokyo"),
		"2026-10-24T01:49:11",
	);
	assert.equal(
		parseZonedInput("2026-10-24T01:49:11", "Asia/Tokyo"),
		"2026-10-23T16:49:11.000Z",
	);
	assert.equal(
		parseZonedInput("2026-10-23T16:49", "UTC"),
		"2026-10-23T16:49:00.000Z",
	);
	assert.equal(parseZonedInput("2026-03-08T02:30:00", "America/New_York"), "");
	assert.equal(
		parseZonedInput("2026-10-23T12:49:11", "America/New_York"),
		"2026-10-23T16:49:11.000Z",
	);
	assert.equal(parseZonedInput("not a date", "UTC"), "");
});
