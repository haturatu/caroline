// biome-ignore lint/correctness/noUnusedImports: TypeScript JSX factory imports are compiler inputs.
import { Fragment, h, type Child } from "./jsx-runtime";

export function AppShell(): Node {
	return (
		<>
			<a className="skip-link" href="#main-content">
				Skip to content
			</a>
			<div className="app-shell" id="appShell">
				<header className="global-header">
					<button
						className="console-menu"
						id="consoleMenuButton"
						type="button"
						aria-label="Open section navigation"
						aria-controls="sideNav"
						aria-expanded="false"
						title="Open section navigation"
					>
						☰
					</button>
					<a className="brand" href="/" aria-label="Caroline home">
						<span className="brand-mark">C</span>
						<span className="brand-name">Caroline</span>
					</a>
					<div className="global-actions">
						<button
							className="header-action"
							id="headerMenuButton"
							type="button"
							popovertarget="headerMenu"
							popovertargetaction="toggle"
							aria-label="Open workspace options"
							aria-controls="headerMenu"
							aria-expanded="false"
							title="Workspace options"
						>
							⋮
						</button>
						<div className="header-menu" id="headerMenu" popover="auto">
							<button
								className="menu-item"
								id="themeToggleButton"
								type="button"
							>
								Use Light Theme
							</button>
							<button
								className="menu-item"
								id="refreshButton"
								type="button"
								data-i18n="common.refresh"
							>
								Refresh Logs Now
							</button>
							<div className="language-menu-section">
								<span className="menu-label" id="languageLabel">
									Language
								</span>
								<button
									className="menu-item language-option"
									type="button"
									data-locale="en"
								>
									English
								</button>
								<button
									className="menu-item language-option"
									type="button"
									data-locale="ja"
								>
									日本語
								</button>
								<button
									className="menu-item language-option"
									type="button"
									data-locale="zh-CN"
								>
									简体中文
								</button>
								<button
									className="menu-item language-option"
									type="button"
									data-locale="zh-TW"
								>
									繁體中文
								</button>
								<button
									className="menu-item language-option"
									type="button"
									data-locale="ru"
								>
									Русский
								</button>
							</div>
						</div>
					</div>
				</header>

				<div className="workspace-layout">
					<nav className="side-nav" id="sideNav" aria-label="Workspace sections">
						<div className="side-nav-scroll">
							<NavigationSection id="exploreNav" labelKey="nav.explore">
								<NavigationItem
									id="logsNavButton"
									labelKey="nav.logsExplorer"
									path="M4 5h16M4 12h16M4 19h10"
									view="explorer"
								/>
								<NavigationItem
									id="timelineNavButton"
									labelKey="nav.timeline"
									path="M4 19V5M4 19h16M8 15V9M12 15V6M16 15v-3"
									section="timeline"
								/>
								<NavigationItem
									id="fieldsNavButton"
									labelKey="nav.fields"
									path="M5 5h14v14H5zM8 9h8M8 13h5"
									section="fields"
										/>
									</NavigationSection>
							<NavigationSection id="detectNav" labelKey="nav.detect">
								<NavigationItem
									id="alertsNavButton"
									labelKey="nav.alerts"
									path="M18 9a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"
									view="alerts"
								/>
							</NavigationSection>
							<NavigationSection id="manageNav" labelKey="nav.manage">
								<NavigationItem
									id="nodesNavButton"
									labelKey="nav.nodes"
									path="M12 3a3 3 0 1 0 0 6 3 3 0 0 0 0-6ZM5 15a3 3 0 1 0 0 6 3 3 0 0 0 0-6Zm14 0a3 3 0 1 0 0 6 3 3 0 0 0 0-6ZM12 9v3M7.5 16.5 11 12M16.5 16.5 13 12"
									view="nodes"
								/>
							</NavigationSection>
						</div>
					</nav>
					<div className="mobile-nav-backdrop" id="mobileNavBackdrop" hidden />

					<main className="main-content" id="main-content" tabIndex={-1}>
						<div id="explorerView">
						<div className="page-heading">
							<div>
								<h1>Logs Explorer</h1>
								<p>Search and inspect Docker container output.</p>
							</div>
							<div className="heading-actions">
								<button
									className="text-button"
									id="createAlertButton"
									type="button"
									data-i18n="alerts.create"
								>
										Create Alert
									</button>
									<button
										className="text-button"
										id="manageAlertsButton"
										type="button"
										data-i18n="alerts.manage"
									>
										Manage Alerts
									</button>
									<button className="text-button" id="shareButton" type="button">
									Share Link
								</button>
							</div>
						</div>

						<QueryPanel />

						<div
							className="error-banner"
							id="errorBanner"
							role="alert"
							aria-live="polite"
							tabIndex={-1}
							hidden
						>
							<span className="error-icon" aria-hidden="true">
								!
							</span>
							<span className="error-content">
								<span id="errorMessage" />
								<details className="error-details" id="errorDetails" hidden>
									<summary>View Details</summary>
									<ul id="errorDetailsList" />
								</details>
							</span>
							<button className="text-button" id="errorDismiss" type="button">
								Dismiss
							</button>
						</div>

						<div className="explorer-grid">
							<FieldsPanel />
							<div className="results-column">
								<TimelinePanel />
								<ResultsPanel />
							</div>
						</div>

						<footer className="page-footer">
							<span>Caroline for Docker Engine</span>
						</footer>
						</div>

						<AlertManagementView />
						<NodeManagementView />
					</main>
				</div>
			</div>

			<dialog
				className="alert-dialog"
				id="alertDialog"
				aria-labelledby="alertDialogTitle"
			>
				<form id="alertForm">
					<div className="dialog-heading">
						<div>
							<div className="section-kicker" data-i18n="alerts.title">
								Log Alerts
							</div>
							<h2 id="alertDialogTitle" data-i18n="alerts.createTitle">
								Create Log Alert
							</h2>
						</div>
						<button
							className="icon-button"
							id="closeAlertButton"
							type="button"
							aria-label="Close"
						>
							×
						</button>
					</div>
					<label className="alert-field">
						<span data-i18n="alerts.name">Name</span>
						<input
							id="alertNameInput"
							name="name"
							required
							data-i18n-placeholder="alerts.namePlaceholder"
							placeholder="e.g. API errors"
						/>
					</label>
					<label className="alert-field alert-query-field">
						<span data-i18n="alerts.query">Query</span>
						<textarea
							className="alert-query-input"
							id="alertQueryInput"
							name="query"
							rows={4}
							spellcheck={false}
							data-i18n-placeholder="alerts.queryPlaceholder"
							placeholder="severity >= ERROR"
							aria-label="Alert query"
							data-i18n-aria-label="alerts.query"
						/>
						<span className="alert-field-hint" data-i18n="alerts.queryHint">
							Use the same query syntax as Logs Explorer. Leave blank for all logs.
						</span>
					</label>
					<div className="alert-number-grid">
						<label className="alert-field">
							<span data-i18n="alerts.severity">Severity</span>
							<select id="alertSeverityInput" name="severity" defaultValue="warning">
								<option value="" data-i18n="alerts.severityNone">Not set</option>
								<option value="info">info</option>
								<option value="warning">warning</option>
								<option value="critical">critical</option>
							</select>
						</label>
						<label className="alert-field">
							<span data-i18n="alerts.sample">Sample</span>
							<select id="alertSampleModeInput" name="sampleMode" defaultValue="summary">
								<option value="off" data-i18n="alerts.sampleOff">Off</option>
								<option value="summary" data-i18n="alerts.sampleSummary">Summary</option>
								<option value="full" data-i18n="alerts.sampleFull">Full (redacted)</option>
							</select>
						</label>
					</div>
					<label className="alert-field">
						<span data-i18n="alerts.labels">Labels</span>
						<input
							id="alertLabelsInput"
							name="labels"
							data-i18n-placeholder="alerts.labelsOptional"
							placeholder="service=api, environment=production"
						/>
					</label>
					<div className="alert-number-grid">
						<label className="alert-field">
							<span data-i18n="alerts.thresholdLabel">
								Matches before firing
							</span>
							<input
								id="alertThresholdInput"
								name="threshold"
								type="number"
								min="1"
								max="1000000"
								defaultValue="1"
								required
							/>
						</label>
						<label className="alert-field">
							<span data-i18n="alerts.window">Window (seconds)</span>
							<input
								id="alertWindowInput"
								name="windowSeconds"
								type="number"
								min="1"
								max="604800"
								defaultValue="60"
								required
							/>
						</label>
						<label className="alert-field">
							<span data-i18n="alerts.cooldown">Cooldown (seconds)</span>
							<input
								id="alertCooldownInput"
								name="cooldownSeconds"
								type="number"
								min="0"
								max="2592000"
								defaultValue="600"
								required
							/>
						</label>
					</div>
					<label className="alert-field">
						<span data-i18n="alerts.runbook">Runbook URL</span>
						<input
							id="alertRunbookInput"
							name="runbookUrl"
							type="url"
							data-i18n-placeholder="alerts.runbookOptional"
							placeholder="Optional runbook URL"
						/>
					</label>
					<label className="alert-field">
						<span data-i18n="alerts.webhook">Webhook URL</span>
						<input
							id="alertWebhookInput"
							name="webhookUrl"
							type="url"
							data-i18n-placeholder="alerts.webhookOptional"
							placeholder="Optional generic webhook"
						/>
					</label>
					<p
						className="alert-field-hint"
						id="alertWebhookHint"
						data-i18n="alerts.webhookKeepHint"
						hidden
					>
						Leave blank to keep the configured webhook.
					</p>
					<label className="alert-checkbox" id="alertRemoveWebhookField" hidden>
						<input id="alertRemoveWebhookInput" type="checkbox" />
						<span data-i18n="alerts.removeWebhook">Remove configured webhook</span>
					</label>
					<div className="dialog-actions">
						<button
							className="text-button"
							id="cancelAlertButton"
							type="button"
							data-i18n="alerts.cancel"
						>
							Cancel
						</button>
						<button
							className="run-button"
							id="saveAlertButton"
							type="submit"
							data-i18n="alerts.save"
						>
							Create Alert
						</button>
					</div>
				</form>
			</dialog>

			<dialog
				className="detail-drawer"
				id="detailDrawer"
				aria-labelledby="detailTitle"
			>
				<div className="detail-drawer-header">
					<div>
						<span className="section-kicker">LOG ENTRY</span>
						<h2 id="detailTitle">Entry Details</h2>
					</div>
					<button
						className="icon-button"
						id="closeDetailButton"
						type="button"
						aria-label="Close Entry Details"
						title="Close Entry Details"
					>
						×
					</button>
				</div>
				<div className="detail-drawer-body" id="detailBody" />
				<div className="detail-drawer-footer" id="detailFooter" hidden />
			</dialog>
			<div
				className="toast"
				id="toast"
				role="status"
				aria-live="polite"
				hidden
			/>
		</>
	);
}

function QueryPanel(): Node {
	return (
		<section className="query-panel" aria-labelledby="query-title">
			<div className="query-panel-header">
				<div>
					<div className="section-kicker">FILTERS</div>
					<h2 id="query-title">Search Logs</h2>
				</div>
				<div className="query-panel-actions">
					<span className="query-shortcut">
						<kbd>Ctrl</kbd>
						<kbd>Enter</kbd> to run advanced query
					</span>
					<button className="text-button" id="clearQueryButton" type="button">
						Reset Filters
					</button>
					<button className="run-button" id="runQueryButton" type="button">
						<span className="run-button-icon" aria-hidden="true">
							<svg viewBox="0 0 24 24" aria-hidden="true">
								<path d="m8 5 8 7-8 7z" />
							</svg>
						</span>
						<span>Run Query</span>
						<span className="button-spinner spinner" aria-hidden="true" />
					</button>
				</div>
			</div>
			<div className="query-search-row">
				<label className="search-all-fields">
					<span aria-hidden="true">⌕</span>
					<span className="sr-only">Search Logs</span>
					<input
						id="searchAllFields"
						type="search"
						autocomplete="off"
						placeholder="Search Logs…"
					/>
				</label>
				<span className="search-hint">
					Press <kbd>/</kbd> to focus
				</span>
			</div>
			<fieldset className="refine-row">
				<legend className="sr-only">Query Filters</legend>
				<FilterControl id="containerFilter" label="Container">
					<option value="">All containers</option>
				</FilterControl>
				<FilterControl id="nodeFilter" label="Node">
					<option value="">All nodes</option>
				</FilterControl>
				<FilterControl id="streamFilter" label="Stream">
					<option value="">All streams</option>
					<option value="stdout">stdout</option>
					<option value="stderr">stderr</option>
				</FilterControl>
				<FilterControl id="severityFilter" label="Severity">
					<option value="">All severities</option>
					<option value="ERROR">Errors</option>
					<option value="WARNING">Warnings</option>
					<option value="INFO">Info</option>
					<option value="DEBUG">Debug</option>
				</FilterControl>
				<FilterControl id="rangeFilter" label="Time" className="time-control">
					<option value="5m">Last 5 minutes</option>
					<option value="15m">Last 15 minutes</option>
					<option value="1h">Last 1 hour</option>
					<option value="6h">Last 6 hours</option>
					<option value="24h">Last 24 hours</option>
					<option value="7d">Last 7 days</option>
					<option value="custom">Selected interval</option>
				</FilterControl>
				<div className="custom-range-editor" id="customRangeEditor" hidden>
					<label>
						<span>From</span>
						<input id="customFromInput" type="datetime-local" step="1" />
					</label>
					<label>
						<span>To</span>
						<input id="customToInput" type="datetime-local" step="1" />
					</label>
					<button
						className="run-button compact-run"
						id="applyCustomRangeButton"
						type="button"
					>
						Apply
					</button>
					<button
						className="text-button"
						id="clearCustomRangeButton"
						type="button"
					>
						Clear Interval
					</button>
				</div>
				<button
					className="text-button show-query-button"
					id="showQueryButton"
					type="button"
				>
					Show Query
				</button>
				<button
					className="stream-button"
					id="streamButton"
					type="button"
					aria-pressed="true"
					title="Live SSE stream"
				>
					<span className="stream-dot" />
					<span id="streamButtonText">Streaming</span>
				</button>
			</fieldset>
			<div className="query-editor" id="queryEditor" hidden>
				<div className="query-editor-heading">
					<strong>Advanced Query</strong>
					<span>Ctrl + Enter to run</span>
				</div>
				<label className="sr-only" htmlFor="queryInput">
					Advanced Logging Query
				</label>
				<textarea
					id="queryInput"
					name="query"
					rows={3}
					spellcheck={false}
					placeholder="severity >= ERROR"
					role="combobox"
					aria-autocomplete="list"
					aria-controls="querySuggestions"
					aria-expanded="false"
				/>
				<div
					className="query-suggestions"
					id="querySuggestions"
					role="listbox"
					aria-label="Query Suggestions"
					hidden
				/>
				<div className="query-combination">
					<span>Combined with basic filters:</span>
					<code id="combinedQueryPreview">All Logs</code>
				</div>
			</div>
			<div className="query-help">
				<span className="info-icon" aria-hidden="true">
					i
				</span>
				<span>
					Basic filters update immediately. Advanced query runs with{" "}
					<kbd>Ctrl</kbd> + <kbd>Enter</kbd>.
				</span>
				<button
					className="text-button"
					id="queryHelpButton"
					type="button"
					popovertarget="queryHelpPopover"
					popovertargetaction="toggle"
					aria-expanded="false"
					aria-controls="queryHelpPopover"
				>
					Query Syntax
				</button>
				<div
					className="query-help-popover"
					id="queryHelpPopover"
					popover="auto"
					role="dialog"
					aria-labelledby="queryHelpTitle"
				>
					<strong id="queryHelpTitle">Caroline Query Syntax</strong>
					<p>Inspired by Google Cloud Logging syntax. Not fully compatible.</p>
					<pre>
						{
							'severity >= ERROR\ncontainer = "nginx"\nstream = "stderr"\nSEARCH("timeout")'
						}
					</pre>
					<p>
						Combine clauses with <code>AND</code> or <code>OR</code>.
					</p>
				</div>
			</div>
		</section>
	);
}

function AlertManagementView(): Node {
	return (
		<section
			className="alert-management-view"
			id="alertsView"
			hidden
			aria-labelledby="alertsPageTitle"
		>
			<div className="page-heading alert-page-heading">
				<div>
					<div className="section-kicker" data-i18n="alerts.kicker">
						ALERTING
					</div>
					<h1 id="alertsPageTitle" tabIndex={-1} data-i18n="alerts.pageTitle">
						Alert policies
					</h1>
					<p data-i18n="alerts.pageDescription">
						Manage rules evaluated against your Docker logs.
					</p>
				</div>
				<div className="heading-actions">
					<button
						className="text-button"
						id="refreshAlertsButton"
						type="button"
						data-i18n="alerts.refresh"
					>
						Refresh Alerts
					</button>
					<button
						className="run-button"
						id="createAlertPageButton"
						type="button"
						data-i18n="alerts.create"
					>
						Create Alert
					</button>
				</div>
			</div>

			<div className="alert-summary-grid" id="alertSummary" aria-live="polite">
				<SummaryCard id="alertSummaryTotal" label="alerts.summaryPolicies" />
				<SummaryCard id="alertSummaryFiring" label="alerts.summaryFiring" />
				<SummaryCard id="alertSummaryEnabled" label="alerts.summaryEnabled" />
				<SummaryCard
					id="alertSummaryNotifications"
					label="alerts.summaryNotifications"
				/>
			</div>

			<div className="alert-info-banner">
				<span className="info-icon" aria-hidden="true">
					i
				</span>
				<div>
					<strong data-i18n="alerts.localEvaluation">
						Evaluated locally from Docker logs
					</strong>
					<p data-i18n="alerts.localEvaluationDescription">
						Rules use the shared log stream. Webhook URLs are never shown in this list.
					</p>
				</div>
			</div>

			<div className="alert-list-panel">
				<div className="alert-list-toolbar">
					<label className="alert-search">
						<span aria-hidden="true">⌕</span>
						<span className="sr-only" data-i18n="alerts.searchLabel">
							Search alert policies
						</span>
						<input
							id="alertSearchInput"
							type="search"
							data-i18n-placeholder="alerts.searchPlaceholder"
							placeholder="Search alert policies…"
						/>
					</label>
					<label className="alert-status-filter">
						<span data-i18n="alerts.statusFilter">Status</span>
						<select
							id="alertStatusFilter"
							aria-label="Alert status"
							data-i18n-aria-label="alerts.statusFilter"
						>
							<option value="all" data-i18n="alerts.allStatuses">All statuses</option>
							<option value="FIRING" data-i18n="alerts.firing">Firing</option>
							<option value="OK" data-i18n="alerts.ok">OK</option>
							<option value="PAUSED" data-i18n="alerts.paused">Paused</option>
						</select>
					</label>
				</div>
				<div className="alert-list-heading">
					<h2 data-i18n="alerts.title">Log Alerts</h2>
					<span id="alertListCount" />
				</div>
				<div id="alertList">
					<p className="alerts-empty" data-i18n="alerts.empty">
						No alert rules yet. Create one from the current query.
					</p>
				</div>
			</div>
		</section>
	);
}

function NodeManagementView(): Node {
	return (
		<section
			className="node-management-view"
			id="nodesView"
			hidden
			aria-labelledby="nodesPageTitle"
		>
			<div className="page-heading node-page-heading">
				<div>
					<div className="section-kicker" data-i18n="nodes.kicker">
						NODES
					</div>
					<h1 id="nodesPageTitle" tabIndex={-1} data-i18n="nodes.pageTitle">
						Nodes
					</h1>
					<p data-i18n="nodes.pageDescription">
						Manage Docker hosts connected through Caroline Agent.
					</p>
				</div>
				<div className="heading-actions">
					<button className="text-button" id="refreshNodesButton" type="button" data-i18n="nodes.refresh">
						Refresh Nodes
					</button>
					<button className="run-button" id="createEnrollmentButton" type="button" data-i18n="nodes.createEnrollment">
						Create Enrollment Token
					</button>
				</div>
			</div>

			<div className="node-info-banner">
				<strong data-i18n="nodes.securityTitle">Agent authentication</strong>
				<p data-i18n="nodes.securityDescription">
					Enrollment tokens are single-use. Agents sign subsequent requests with their persistent key.
				</p>
			</div>
			<div className="node-enrollment-result" id="nodeEnrollmentResult" hidden>
				<div>
					<strong data-i18n="nodes.tokenCreated">Enrollment token created</strong>
					<p id="nodeEnrollmentExpires" />
				</div>
				<div className="node-enrollment-details">
					<div>
						<span data-i18n="nodes.tokenLabel">Token</span>
						<code id="nodeEnrollmentToken" />
					</div>
					<div>
						<span data-i18n="nodes.composeURL">Compose enrollment URL</span>
						<code id="nodeEnrollmentURL" />
					</div>
				</div>
				<div className="node-enrollment-actions">
					<button className="text-button" id="copyEnrollmentButton" type="button" data-i18n="nodes.copyToken">
						Copy Token
					</button>
					<button className="text-button" id="copyEnrollmentURLButton" type="button" data-i18n="nodes.copyEnrollmentURL">
						Copy URL
					</button>
				</div>
			</div>
			<div className="node-list-panel">
				<div className="node-list-heading">
					<h2 data-i18n="nodes.connectedHosts">Connected hosts</h2>
					<span id="nodeListCount" />
				</div>
				<div id="nodeList">
					<p className="nodes-empty" data-i18n="nodes.loading">Loading nodes…</p>
				</div>
			</div>
		</section>
	);
}

function SummaryCard({ id, label }: { id: string; label: string }): Node {
	return (
		<div className="alert-summary-card">
			<span data-i18n={label}>{label}</span>
			<strong id={id}>—</strong>
			<small id={`${id}Description`} />
		</div>
	);
}

function NavigationSection({
	id,
	labelKey,
	children,
}: {
	id: string;
	labelKey: string;
	children: Child | Child[];
}): Node {
	return (
		<section className="nav-section" data-nav-section>
			<button
				className="nav-section-toggle"
				type="button"
				aria-expanded="true"
				aria-controls={`${id}Items`}
			>
				<span data-i18n={labelKey}>{labelKey}</span>
				<svg className="nav-chevron" viewBox="0 0 24 24" aria-hidden="true">
					<path d="m6 15 6-6 6 6" />
				</svg>
			</button>
			<div className="nav-section-items" id={`${id}Items`}>
				{children}
			</div>
		</section>
	);
}

function NavigationItem({
	id,
	labelKey,
	path,
	view,
	section,
}: {
	id: string;
	labelKey: string;
	path: string;
	view?: "explorer" | "alerts" | "nodes";
	section?: "timeline" | "fields";
}): Node {
	return (
		<button
			className={`side-nav-link${view === "explorer" ? " active" : ""}`}
			id={id}
			type="button"
			aria-current={view === "explorer" ? "location" : undefined}
			data-nav-view={view}
			data-nav-section-target={section}
		>
			<svg viewBox="0 0 24 24" aria-hidden="true">
				<path d={path} />
			</svg>
			<span data-i18n={labelKey}>{labelKey}</span>
		</button>
	);
}

type FilterControlProps = {
	children?: Child | Child[];
	id: string;
	label: string;
	className?: string;
};

function FilterControl({
	children,
	id,
	label,
	className,
}: FilterControlProps): Node {
	return (
		<label className={`filter-control${className ? ` ${className}` : ""}`}>
			<span>{label}</span>
			<select
				id={id}
				name={id === "rangeFilter" ? "duration" : id.replace("Filter", "")}
			>
				{children}
			</select>
		</label>
	);
}

function FieldsPanel(): Node {
	return (
		<aside
			className="fields-panel"
			id="fields"
			tabIndex={-1}
			aria-labelledby="fields-title"
		>
			<div className="panel-heading">
				<h2 id="fields-title">Fields</h2>
				<button
					className="icon-button compact"
					id="fieldsToggle"
					type="button"
					aria-label="Hide Fields"
					title="Hide Fields"
				>
					‹
				</button>
			</div>
			<div id="fieldGroups">
				<div className="panel-loading">
					Fields appear after you run a query.
				</div>
			</div>
		</aside>
	);
}

function TimelinePanel(): Node {
	return (
		<section
			className="timeline-panel"
			id="timeline"
			tabIndex={-1}
			aria-labelledby="timeline-title"
		>
			<div className="panel-heading">
				<h2 id="timeline-title">Timeline</h2>
				<div className="timeline-actions">
					<button
						className="icon-button compact timeline-zoom"
						id="timelineZoomOut"
						type="button"
						aria-label="Zoom Out Timeline"
						title="Zoom Out Timeline"
					>
						<svg viewBox="0 0 24 24" aria-hidden="true">
							<circle cx="10.5" cy="10.5" r="5.5" />
							<path d="m15 15 4 4M8 10.5h5" />
						</svg>
					</button>
					<button
						className="icon-button compact timeline-zoom"
						id="timelineZoomIn"
						type="button"
						aria-label="Zoom In Timeline"
						title="Zoom In Timeline"
					>
						<svg viewBox="0 0 24 24" aria-hidden="true">
							<circle cx="10.5" cy="10.5" r="5.5" />
							<path d="m15 15 4 4M8 10.5h5M10.5 8v5" />
						</svg>
					</button>
					<span className="timeline-divider" aria-hidden="true" />
					<button
						className="icon-button compact"
						id="timelineToggle"
						type="button"
						aria-label="Collapse Timeline"
						title="Collapse Timeline"
					>
						⌃
					</button>
				</div>
			</div>
			<div className="timeline-chart" id="timelineChart">
				<div className="chart-loading">Loading Timeline…</div>
			</div>
			<fieldset className="timeline-legend" id="timelineLegend">
				<legend className="sr-only">Timeline Severity Legend</legend>
			</fieldset>
			<div className="timeline-axis" id="timelineAxis" />
		</section>
	);
}

function ResultsPanel(): Node {
	return (
		<section className="results-panel" aria-labelledby="results-title">
			<div className="results-header">
				<div>
					<div className="results-title-row">
						<h2 id="results-title">Logs Explorer</h2>
						<output className="result-count" id="resultCount">
							—
						</output>
						<span
							className="approximate-badge"
							id="approximateBadge"
							title="Latest 1,000 lines per container"
						>
							Partial
						</span>
						<output
							className="refresh-status"
							id="refreshStatus"
							aria-live="polite"
						/>
					</div>
					<p id="resultsDescription" />
				</div>
				<div className="results-actions">
					<button
						className="text-button"
						id="wrapButton"
						type="button"
						aria-pressed="false"
					>
						Wrap Lines
					</button>
					<select
						className="sort-select"
						id="sortFilter"
						aria-label="Sort Results"
					>
						<option value="desc">Newest First</option>
						<option value="asc">Oldest First</option>
					</select>
				</div>
			</div>
			<div className="results-table-scroll">
				<div className="results-table-head">
					<span>TIME</span>
					<span>SEVERITY</span>
					<span>RESOURCE</span>
					<span>SUMMARY</span>
					<span />
				</div>
				<div className="entry-list" id="entryList">
					<div className="empty-state">
						<span className="empty-state-icon" aria-hidden="true">
							⌕
						</span>
						<strong>Run a Query</strong>
						<p>Your matching log entries will appear here.</p>
					</div>
				</div>
			</div>
			<div className="results-footer">
				<span id="resultsFooter" />
				<button
					className="text-button"
					id="nextPageButton"
					type="button"
					hidden
				>
					Load More
				</button>
			</div>
		</section>
	);
}
