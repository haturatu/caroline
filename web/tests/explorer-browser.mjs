import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, resolve, sep } from "node:path";
import { once } from "node:events";
import { chromium } from "playwright-core";

const root = resolve("static");
const streams = new Set();
const server = createServer(async (request, response) => {
	if (request.url.startsWith("/api/tail")) {
		response.writeHead(200, {
			"Content-Type": "text/event-stream",
			"Cache-Control": "no-cache",
		});
		response.write(": connected\n\n");
		streams.add(response);
		response.on("close", () => streams.delete(response));
		return;
	}
	const path = resolve(
		root,
		"." + new URL(request.url, "http://localhost").pathname,
	);
	if (path !== root && !path.startsWith(root + sep)) {
		response.writeHead(403).end();
		return;
	}
	try {
		const file =
			path === root || request.url.startsWith("/?")
				? resolve(root, "index.html")
				: path;
		response.setHeader(
			"Content-Type",
			{
				".js": "application/javascript",
				".css": "text/css",
				".html": "text/html",
			}[extname(file)] || "application/octet-stream",
		);
		response.end(await readFile(file));
	} catch {
		response.writeHead(404).end();
	}
});
server.listen(0, "127.0.0.1");
await once(server, "listening");
const browser = await chromium.connectOverCDP(
	process.env.CDP_URL || "http://localhost:9222",
);
const context = await browser.newContext({
	viewport: { width: 1440, height: 1000 },
	locale: "en-US",
	timezoneId: "Asia/Tokyo",
	acceptDownloads: true,
});
const page = await context.newPage();
page.setDefaultTimeout(15000);
const errors = [];
page.on("pageerror", (error) => errors.push(error.message));
const requests = [];
let delayRequest = false;
const entries = Array.from({ length: 7 }, (_, index) => ({
	insertId: `entry-${index}`,
	timestamp: `2026-10-23T16:49:${String(11 - index).padStart(2, "0")}.005Z`,
	severity: index === 0 ? "ERROR" : "INFO",
	stream: index === 0 ? "stderr" : "stdout",
	logName: `containers/api/${index === 0 ? "stderr" : "stdout"}`,
	resource: {
		type: "docker_container",
		labels: {
			container_id: "container-one",
			container_name: "api",
			node_id: "node-one",
			node_name: "host-one",
		},
	},
	labels: { team: "platform" },
	summary:
		index === 0 ? "timeout <script>alert(1)</script>" : "request completed",
	textPayload:
		index === 0 ? "timeout <script>alert(1)</script>" : "request completed",
	jsonPayload: {
		message: index === 0 ? 'say "hello" AND goodbye' : "request completed",
		status: 500,
		nested: { region: "ap-northeast-1" },
	},
}));
await context.route("**/api/**", async (route) => {
	const url = new URL(route.request().url());
	if (url.pathname === "/api/tail") return route.continue();
	if (url.pathname === "/api/status")
		return route.fulfill({ json: { connected: true, dockerVersion: "test" } });
	if (url.pathname === "/api/alerts")
		return route.fulfill({ json: { rules: [] } });
	if (url.pathname === "/api/nodes")
		return route.fulfill({ json: { nodes: [] } });
	if (url.pathname === "/api/explorer") {
		requests.push(url);
		if (delayRequest) await new Promise((resolve) => setTimeout(resolve, 1000));
		return route
			.fulfill({
				json: {
					entries: url.searchParams.has("pageToken")
						? [{ ...entries[0], insertId: "next-page-entry" }]
						: entries,
					containers: [
						{
							id: "container-one",
							name: "api",
							nodeId: "node-one",
							nodeName: "host-one",
							image: "api:test",
							status: "running",
							logCount: 7,
							errorCount: 1,
							warningCount: 0,
						},
					],
					timeline: [
						{
							start: "2026-10-23T16:45:00Z",
							end: "2026-10-23T16:50:00Z",
							total: 7,
							severities: { ERROR: 1, INFO: 6 },
						},
					],
					fields: [
						{
							name: "System Metadata",
							fields: [
								{ name: "severity", count: 7, values: { INFO: 6, ERROR: 1 } },
								{
									name: "logName",
									count: 7,
									values: {
										"containers/api/stderr": 1,
										"containers/api/stdout": 6,
									},
								},
								{
									name: "resource.type",
									count: 7,
									values: { docker_container: 7 },
								},
								{
									name: "resource.labels.container_name",
									count: 7,
									values: { api: 7 },
								},
							],
						},
						{
							name: "Frequent Fields",
							fields: [
								{ name: "jsonPayload.status", count: 7, values: { "500": 7 } },
								{
									name: "jsonPayload.message",
									count: 7,
									values: Object.fromEntries(
										Array.from({ length: 7 }, (_, i) => [`value-${i}`, 7 - i]),
									),
								},
							],
						},
					],
					total: 8,
					nextPageToken: url.searchParams.has("pageToken") ? "" : "second-page",
					generatedAt: "2026-10-23T16:50:00Z",
					from: url.searchParams.get("from") || "2026-10-23T16:45:00Z",
					to: url.searchParams.get("to") || "2026-10-23T16:50:00Z",
					duration: "5m",
					query: url.searchParams.get("q") || "",
					approximate: false,
					logTail: 1000,
					entryLimit: 50000,
					truncated: false,
				},
			})
			.catch(() => {});
	}
	return route.fulfill({ json: {} });
});
const base = `http://127.0.0.1:${server.address().port}`;
async function loaded() {
	await page.locator(".entry-row").first().waitFor();
	await page.waitForFunction(
		() =>
			document.querySelector("#runQueryButton")?.getAttribute("aria-busy") ===
			"false",
	);
}
try {
	await page.goto(base + "/?live=0&fields=1");
	await loaded();
	assert.equal(await page.locator(".entry-row").count(), 7);
	assert.equal(
		await page
			.locator(".entry-record")
			.first()
			.evaluate((el) => getComputedStyle(el).gridColumn),
		"1 / -1",
	);
	await page.locator(".entry-row").first().evaluate(el => el.scrollIntoView({block:"center"}));
	assert.ok(await page.locator(".entry-time").first().isVisible());
	assert.equal(
		await page.locator("script").filter({ hasText: "alert(1)" }).count(),
		0,
	);
	await page.locator("#resultHighlight").fill("timeout");
	assert.equal(
		await page.locator(".entry-summary mark").first().textContent(),
		"timeout",
	);
	await page.locator("[data-expand-entry]").first().click();
	await page.locator(".inline-entry .log-tree").waitFor();
	await page.locator('.inline-entry [data-log-action="expand"]').click();
	assert.ok(
		await page
			.locator(
				'.inline-entry [data-path="jsonPayload.message"][data-log-action="include"]',
			)
			.count(),
	);
	await page.locator(".entry-row").first().click();
	await page.locator("#detailDrawer[open]").waitFor();
	await page.locator('#detailBody [data-log-action="expand"]').click();
	const action = page.locator(
		'#detailBody [data-log-action="include"][data-path="jsonPayload.message"]',
	);
	await action.locator("..").locator("..").locator("summary").click();
	await action.click();
	await loaded();
	assert.equal(await page.locator("#detailDrawer").getAttribute("open"), null);
	assert.equal(
		requests.at(-1).searchParams.get("q"),
		"jsonPayload.message = " + JSON.stringify('say "hello" AND goodbye'),
	);
	console.log(
		"PASS: rows, XSS escaping, highlight, inline tree, include filter",
	);
	await page.locator("#saveQueryButton").click();
	await page.locator("#savedQueryName").fill("Production Errors");
	await page.locator("#saveQueryForm button[type=submit]").click();
	await page.locator("#queryLibraryButton").click();
	await page.locator('[data-library-tab="saved"]').click();
	assert.ok(
		(await page.locator("#queryLibraryList").innerText()).includes(
			"Production Errors",
		),
	);
	await page.keyboard.press("Escape");
	await page.locator("#clearQueryButton").click();
	await loaded();
	await page.locator("#queryLibraryButton").click();
	await page.locator('[data-library-tab="saved"]').click();
	await page.locator("[data-query-run]").click();
	await loaded();
	assert.ok(
		requests.at(-1).searchParams.get("q").includes("jsonPayload.message"),
	);
	console.log("PASS: save, find and execute a query");
	await page.locator("#previousTimeButton").click();
	await loaded();
	assert.equal(
		requests.at(-1).searchParams.get("from"),
		"2026-10-23T16:40:00.000Z",
	);
	assert.equal(
		requests.at(-1).searchParams.get("to"),
		"2026-10-23T16:45:00.000Z",
	);
	await page.locator("#timeZoneSelect").selectOption("UTC");
	assert.equal(
		await page.locator("#customFromInput").inputValue(),
		"2026-10-23T16:40",
	);
	await page.locator("#timeZoneSelect").selectOption("Asia/Tokyo");
	assert.equal(
		await page.locator("#customFromInput").inputValue(),
		"2026-10-24T01:40",
	);
	await page.locator("#nextTimeButton").click();
	await loaded();
	await page.goBack();
	await loaded();
	assert.equal(
		new URL(page.url()).searchParams.get("from"),
		"2026-10-23T16:40:00.000Z",
	);
	await page.goForward();
	await loaded();
	assert.equal(new URL(page.url()).searchParams.get("tz"), "Asia/Tokyo");
	console.log(
		"PASS: interval navigation, timezone-aware custom ranges and browser history",
	);
	await page.locator("#fieldSearch").fill("jsonPayload.message");
	assert.equal(await page.locator('[data-filter-field="severity"]').count(), 0);
	await page.locator("#fieldSearch").fill("");
	await page.locator('[data-field="jsonPayload.message"]').click();
	await page.locator("[data-more-values]").click();
	assert.equal(
		await page
			.locator('[data-filter-field="jsonPayload.message"]:not([data-exclude])')
			.count(),
		7,
	);
	await page
		.locator('[data-filter-field="jsonPayload.message"][data-exclude]')
		.first()
		.click();
	await loaded();
	assert.ok(requests.at(-1).searchParams.get("q").includes('!= "value-0"'));
	console.log("PASS: field search, all values and exclude filter");
	await page.locator("#displaySettingsButton").click();
	await page
		.locator("#summaryFieldsInput")
		.fill("jsonPayload.status\nresource.labels.node_name");
	await page.locator("#displayCompact").check();
	await page.locator("#fieldsWidth").evaluate((el) => {
		el.value = "300";
	});
	await page.locator("#chartHeight").evaluate((el) => {
		el.value = "180";
	});
	await page.locator("#displayForm button[type=submit]").click();
	assert.ok(
		(await page.locator(".entry-summary").first().innerText()).includes(
			"jsonPayload.status: 500",
		),
	);
	assert.equal(
		await page
			.locator("body")
			.evaluate((el) => el.classList.contains("compact-logs")),
		true,
	);
	await page.reload();
	await loaded();
	assert.ok(
		(await page.locator(".entry-summary").first().innerText()).includes(
			"jsonPayload.status: 500",
		),
	);
	assert.equal(
		await page.evaluate(() =>
			document.documentElement.style.getPropertyValue("--fields-width"),
		),
		"300px",
	);
	assert.equal(
		await page.evaluate(() =>
			document.documentElement.style.getPropertyValue("--chart-height"),
		),
		"180px",
	);
	console.log(
		"PASS: custom summary, compact rows and panel sizes survive reload",
	);
	await page.locator("#nextPageButton").click();
	await loaded();
	assert.equal(await page.locator(".entry-row").count(), 8);
	await page.locator("#downloadButton").click();
	await page.locator("#downloadFormat").selectOption("json");
	const downloadPromise = page.waitForEvent("download");
	await page.locator("#downloadResults").click();
	const download = await downloadPromise;
	const data = JSON.parse(await readFile(await download.path(), "utf8"));
	assert.equal(data.length, 8);
	console.log("PASS: pagination and JSON download of loaded entries");
	await page.locator(".entry-row").first().click();
	await page.locator('#detailBody [data-log-action="next"]').click();
	assert.equal(new URL(page.url()).searchParams.get("entry"), "entry-1");
	await page.reload();
	await loaded();
	await page.locator("#detailDrawer[open]").waitFor();
	assert.equal(new URL(page.url()).searchParams.get("entry"), "entry-1");
	await page.locator('#detailBody [data-log-action="context"]').click();
	await loaded();
	assert.equal(requests.at(-1).searchParams.get("containers"), "container-one");
	assert.equal(requests.at(-1).searchParams.get("nodes"), "node-one");
	assert.equal(
		requests.at(-1).searchParams.get("q"),
		'resource.labels.container_name = "api"',
	);
	console.log("PASS: entry navigation, deep links and surrounding logs");
	delayRequest = true;
	await page.locator("#runQueryButton").click();
	await page.locator("#cancelQueryButton").click();
	await page.waitForFunction(
		() =>
			document.querySelector("#runQueryButton").getAttribute("aria-busy") ===
			"false",
	);
	delayRequest = false;
	assert.equal(await page.locator("#errorBanner").isVisible(), false);
	console.log("PASS: query cancellation");
	await page.locator("#streamButton").click();
	await page.waitForFunction(() =>
		document
			.querySelector("#refreshStatus")
			.textContent.toLowerCase()
			.includes("live stream connected"),
	);
	await page.locator("#collapseAllButton").click();
	await page.locator("[data-expand-entry]").first().click();
	await page.locator('.inline-entry [data-log-action="expand"]').click();
	await page.locator(".entry-row").first().click();
	await page.locator('#detailBody [data-log-action="expand"]').click();
	await page.locator('#detailBody [data-log-action="raw"]').click();
	for (const response of streams)
		response.write(
			"event: log\ndata: " +
				JSON.stringify({
					...entries[0],
					insertId: "live-entry",
					timestamp: "2026-10-23T16:51:00Z",
				}) +
				"\n\n",
		);
	await page.waitForFunction(
		() => document.querySelectorAll(".entry-row").length === 8,
	);
	assert.equal(await page.locator("#detailBody .entry-raw").isVisible(), true);
	assert.equal(
		await page.locator(".inline-entry .log-tree-node:not([open])").count(),
		0,
	);
	await page.keyboard.press("Escape");
	await page.locator("#streamButton").click();
	console.log("PASS: live updates preserve raw detail and inline expansion");
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto(base + "/?live=0&fields=0");
	await loaded();
	assert.equal(
		await page.evaluate(
			() => document.documentElement.scrollWidth > innerWidth,
		),
		false,
	);
	await page.locator("#displaySettingsButton").click();
	assert.equal(
		await page
			.locator("#displayDialog")
			.evaluate((el) => el.scrollWidth > el.clientWidth),
		false,
	);
	await page.keyboard.press("Escape");
	await page.locator(".entry-row").first().click();
	assert.equal(
		await page
			.locator("#detailDrawer")
			.evaluate((el) => el.scrollWidth > el.clientWidth),
		false,
	);
	await page.keyboard.press("Escape");
	await page.setViewportSize({ width: 1920, height: 1080 });
	await page.goto(base + "/?live=0&fields=1");
	await loaded();
	await page.locator(".entry-row").first().evaluate(el => el.scrollIntoView({block:"center"}));
	assert.equal(
		await page.evaluate(
			() => document.documentElement.scrollWidth > innerWidth,
		),
		false,
	);
	assert.deepEqual(errors, []);
	console.log("PASS: mobile, wide desktop and browser runtime errors");
} finally {
	for (const response of streams) response.end();
	await context.close();
	await browser.close();
	await new Promise((resolve) => server.close(resolve));
}
