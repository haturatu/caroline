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
					<nav className="side-nav" id="sideNav" aria-label="Log sections">
						<div className="side-nav-group">
							<button
								className="side-nav-link active"
								id="logsNavButton"
								type="button"
								aria-current="location"
								title="Logs Explorer"
							>
								<svg viewBox="0 0 24 24" aria-hidden="true">
									<path d="M4 5h16M4 12h16M4 19h10" />
								</svg>
								<span>Logs Explorer</span>
							</button>
							<button
								className="side-nav-link"
								id="timelineNavButton"
								type="button"
								title="Timeline"
							>
								<svg viewBox="0 0 24 24" aria-hidden="true">
									<path d="M4 19V5M4 19h16M8 15V9M12 15V6M16 15v-3" />
								</svg>
								<span>Timeline</span>
							</button>
							<button
								className="side-nav-link"
								id="fieldsNavButton"
								type="button"
								title="Fields"
							>
								<svg viewBox="0 0 24 24" aria-hidden="true">
									<path d="M5 5h14v14H5zM8 9h8M8 13h5" />
								</svg>
								<span>Fields</span>
							</button>
						</div>
						<div className="side-nav-footer">
							<span className="engine-icon" aria-hidden="true">
								D
							</span>
							<span>
								<strong id="sideEngineStatus">Docker Engine</strong>
								<small id="sideEngineVersion">Waiting for connection</small>
							</span>
						</div>
					</nav>
					<div className="mobile-nav-backdrop" id="mobileNavBackdrop" hidden />

					<main className="main-content" id="main-content" tabIndex={-1}>
						<div className="page-heading">
							<div>
								<h1>Logs Explorer</h1>
								<p>Search and inspect Docker container output.</p>
							</div>
							<div className="heading-actions">
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
							<span>
								<span className="footer-dot" />
								<span id="localDataLabel">Data stays on this host</span>
							</span>
						</footer>
					</main>
				</div>
			</div>

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
				<div id="detailBody" />
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
