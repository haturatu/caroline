import { getJSON, requestJSON } from "../../shared/api/http";
import type { NodeInfo } from "../../shared/types";

export type EnrollmentResponse = {
	token: string;
	enrollment: {
		id: string;
		expiresAt: string;
		createdAt: string;
	};
};

export function fetchNodes(): Promise<NodeInfo[]> {
	return getJSON<NodeInfo[]>("/api/nodes");
}

export function createEnrollment(ttlSeconds = 900): Promise<EnrollmentResponse> {
	return requestJSON<EnrollmentResponse>("/api/nodes", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ ttlSeconds }),
	});
}

export function revokeNode(id: string): Promise<{ id: string; status: string }> {
	return requestJSON<{ id: string; status: string }>(
		`/api/nodes/${encodeURIComponent(id)}`,
		{ method: "DELETE" },
	);
}
