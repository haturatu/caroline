import {
	maxTimelineBuckets,
	minTimelineBuckets,
	setTimelineBuckets,
} from "../explorer/api";

function timelineBucketCount(width: number): number {
	return Math.min(
		maxTimelineBuckets,
		Math.max(minTimelineBuckets, Math.round(width / 28)),
	);
}

export function setupTimelineResolution(
	onResolutionChange: () => void,
): () => void {
	const chart = document.querySelector<HTMLElement>("#timelineChart");
	if (!chart) return () => {};

	let currentBucketCount = 0;
	let resizeTimer: number | null = null;
	let initialized = false;
	let ready = false;
	const update = (width: number): void => {
		const nextBucketCount = timelineBucketCount(width || window.innerWidth);
		if (nextBucketCount === currentBucketCount) return;
		currentBucketCount = nextBucketCount;
		setTimelineBuckets(nextBucketCount);
		if (!initialized) {
			initialized = true;
			return;
		}
		if (!ready) return;
		if (resizeTimer !== null) window.clearTimeout(resizeTimer);
		resizeTimer = window.setTimeout(() => {
			resizeTimer = null;
			onResolutionChange();
		}, 180);
	};

	update(chart.getBoundingClientRect().width);
	if (typeof ResizeObserver === "undefined") return () => {
		ready = true;
	};
	const observer = new ResizeObserver(([entry]) => {
		if (entry) update(entry.contentRect.width);
	});
	observer.observe(chart);
	return () => {
		ready = true;
	};
}
