// API client for OCM admin endpoints

const BASE_URL = '/admin/api';

export interface Credential {
	id: string;
	service: string;
	displayName: string;
	type: string;
	scopes: Record<string, Scope>;
	createdAt: string;
	updatedAt: string;
}

export interface Scope {
	name: string;
	envVar?: string;
	permanent: boolean;
	requiresApproval: boolean;
	maxTTL: number;
	token?: string;
	refreshToken?: string;
	expiresAt?: string;
}

export interface Elevation {
	id: string;
	service: string;
	scope: string;
	reason: string;
	status: string;
	requestedAt: string;
	approvedAt?: string;
	expiresAt?: string;
	approvedBy?: string;
}

export interface AuditEntry {
	id: string;
	timestamp: string;
	action: string;
	service?: string;
	scope?: string;
	details?: string;
	actor: string;
}

export interface DashboardData {
	totalCredentials: number;
	pendingRequests: number;
	activeElevations: number;
	recentAudit: AuditEntry[];
	pending: Elevation[];
}

export interface SetupStatus {
	setupComplete: boolean;
	missingKeys: string[];
	configuredKeys: string[];
}

export interface PendingDevice {
	requestId: string;
	deviceId: string;
	role: string;
	origin: string;
	userAgent: string;
	createdAt: number;
}

export interface PairedDevice {
	deviceId: string;
	role: string;
	createdAt: number;
}

export interface DeviceList {
	pending: PendingDevice[];
	paired: PairedDevice[];
}

export interface CreateCredentialRequest {
	service: string;
	displayName: string;
	type: string;
	scopes: Record<string, {
		envVar: string;
		permanent: boolean;
		requiresApproval: boolean;
		maxTTL: string;
		token: string;
		refreshToken?: string;
	}>;
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
	const response = await fetch(`${BASE_URL}${path}`, {
		...options,
		headers: {
			'Content-Type': 'application/json',
			...options?.headers
		}
	});

	if (!response.ok) {
		const error = await response.json().catch(() => ({ error: 'Unknown error' }));
		throw new Error(error.error || `HTTP ${response.status}`);
	}

	return response.json();
}

export const api = {
	// Setup
	getSetupStatus: () => request<SetupStatus>('/setup/status'),
	completeSetup: () => request<{ status: string; message: string }>('/setup/complete', { method: 'POST' }),

	// Dashboard
	getDashboard: () => request<DashboardData>('/dashboard'),

	// Credentials
	listCredentials: () => request<Credential[]>('/credentials'),
	getCredential: (service: string) => request<Credential>(`/credentials/${service}`),
	createCredential: (data: CreateCredentialRequest) =>
		request<Credential>('/credentials', {
			method: 'POST',
			body: JSON.stringify(data)
		}),
	updateCredential: (service: string, data: CreateCredentialRequest) =>
		request<Credential>(`/credentials/${service}`, {
			method: 'PUT',
			body: JSON.stringify(data)
		}),
	deleteCredential: (service: string) =>
		request<void>(`/credentials/${service}`, { method: 'DELETE' }),

	// Elevation requests
	listPendingRequests: () => request<Elevation[]>('/requests'),
	approveRequest: (id: string, ttl: string = '30m') =>
		request<{ status: string; expiresAt: string }>(`/requests/${id}/approve`, {
			method: 'POST',
			body: JSON.stringify({ ttl })
		}),
	denyRequest: (id: string) =>
		request<{ status: string }>(`/requests/${id}/deny`, { method: 'POST' }),
	revokeElevation: (service: string, scope: string) =>
		request<{ status: string }>(`/revoke/${service}/${scope}`, { method: 'POST' }),

	// Audit
	listAuditEntries: (service?: string) =>
		request<AuditEntry[]>(`/audit${service ? `?service=${service}` : ''}`),

	// Device pairing
	listDevices: () => request<DeviceList>('/devices'),
	approveDevice: (requestId: string) =>
		request<{ status: string; requestId: string }>(`/devices/${requestId}/approve`, { method: 'POST' }),
	rejectDevice: (requestId: string) =>
		request<{ status: string; requestId: string }>(`/devices/${requestId}/reject`, { method: 'POST' })
};
